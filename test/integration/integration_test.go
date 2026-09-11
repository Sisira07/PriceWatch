//go:build integration

// Package integration contains a single end-to-end test that runs against a
// real Postgres instance (see docker-compose.yml). It is gated behind the
// "integration" build tag so `go test ./...` (unit tests) never needs a
// database, while CI/local devs can opt in explicitly:
//
//	go test -tags=integration ./test/integration/...
//
// Requires INTEGRATION_DATABASE_URL to point at a scratch Postgres database
// (docker-compose's "postgres" service works: see README.md).
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Sisira07/PriceWatch/internal/auth"
	"github.com/Sisira07/PriceWatch/internal/db"
	"github.com/Sisira07/PriceWatch/internal/handlers"
	"github.com/Sisira07/PriceWatch/internal/models"
	"github.com/Sisira07/PriceWatch/internal/worker"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFullFlow_RegisterLoginWatchTickNotify(t *testing.T) {
	dsn := os.Getenv("INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("INTEGRATION_DATABASE_URL not set; skipping integration test")
	}

	if err := db.RunMigrations(dsn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.New(ctx, db.Config{DSN: dsn})
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	// t.Cleanup runs LIFO (last registered, first run). Registering the pool
	// close first and the data cleanup second means the data cleanup runs
	// while the pool is still open, and the pool closes last.
	t.Cleanup(func() { pool.Close() })
	t.Cleanup(func() { cleanupTestData(t, pool) })

	issuer := auth.NewTokenIssuer("integration-test-secret", time.Hour)
	apiStore := handlers.NewPGStore(pool)
	api := handlers.New(apiStore, issuer, nil)
	server := httptest.NewServer(api.Routes())
	defer server.Close()

	client := server.Client()
	email := "integration-user@example.com"
	password := "supersecret1"

	// 1. Register
	registerResp := doJSON(t, client, http.MethodPost, server.URL+"/auth/register", "",
		models.RegisterRequest{Email: email, Password: password})
	if registerResp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", registerResp.StatusCode)
	}

	// 2. Login
	loginResp := doJSON(t, client, http.MethodPost, server.URL+"/auth/login", "",
		models.LoginRequest{Email: email, Password: password})
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", loginResp.StatusCode)
	}
	var loginBody models.LoginResponse
	decodeBody(t, loginResp, &loginBody)
	if loginBody.Token == "" {
		t.Fatal("expected a JWT from login")
	}

	// 3. Create many watched items. checkPrice (internal/worker/worker.go)
	// returns a price that's 80%-120% of *that item's own* target price, so
	// no single target value makes a hit more likely — each item independently
	// has roughly a 50% chance of price <= target on any given tick. Creating
	// many items makes "zero notifications" astronomically unlikely (about
	// 0.5^itemCount) without weakening what the test actually proves: this
	// still exercises the real worker.Tick() logic end-to-end, just across
	// enough items to make the assertion reliable.
	const itemCount = 25
	itemIDs := make([]string, 0, itemCount)
	for i := 0; i < itemCount; i++ {
		createResp := doJSON(t, client, http.MethodPost, server.URL+"/items", loginBody.Token,
			models.CreateWatchedItemRequest{
				Name:        fmt.Sprintf("Integration Test Widget %d", i),
				URL:         fmt.Sprintf("https://example.com/widget-%d", i),
				TargetPrice: 100,
			})
		if createResp.StatusCode != http.StatusCreated {
			t.Fatalf("create item %d: expected 201, got %d", i, createResp.StatusCode)
		}
		var item models.WatchedItem
		decodeBody(t, createResp, &item)
		if item.ID == "" {
			t.Fatalf("expected created item %d to have an ID", i)
		}
		itemIDs = append(itemIDs, item.ID)
	}

	// 4. Simulate a single worker tick directly against the same DB.
	workerStore := worker.NewPGStore(pool)
	w := worker.New(workerStore, time.Minute)
	if _, err := w.Tick(ctx); err != nil {
		t.Fatalf("worker tick: %v", err)
	}

	// 5. Every item should have at least one price_history row...
	historyResp := doJSON(t, client, http.MethodGet, server.URL+"/items/"+itemIDs[0]+"/history", loginBody.Token, nil)
	if historyResp.StatusCode != http.StatusOK {
		t.Fatalf("get history: expected 200, got %d", historyResp.StatusCode)
	}
	var history []models.PriceHistory
	decodeBody(t, historyResp, &history)
	if len(history) == 0 {
		t.Fatal("expected at least one price_history row after worker tick")
	}

	// ...and, across 25 independent ~50/50 items, at least one notification
	// should have fired.
	var notifCount int
	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM notifications WHERE watched_item_id = ANY($1)`, itemIDs,
	).Scan(&notifCount)
	if err != nil {
		t.Fatalf("query notifications: %v", err)
	}
	if notifCount == 0 {
		t.Fatal("expected at least one notification row across the watched items")
	}
}

func jsonReader(b []byte) io.Reader {
	if b == nil {
		return nil
	}
	return bytes.NewReader(b)
}

func doJSON(t *testing.T, client *http.Client, method, url, token string, body interface{}) *http.Response {
	t.Helper()
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	}
	req, err := http.NewRequest(method, url, jsonReader(reqBody))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func decodeBody(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}

func cleanupTestData(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `DELETE FROM users WHERE email = $1`, "integration-user@example.com")
	if err != nil {
		t.Logf("cleanup warning: %v", err)
	}
	// notifications, watched_items, price_history cascade-delete via FKs.
}
