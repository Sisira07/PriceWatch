package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sisira07/PriceWatch/internal/auth"
	"github.com/Sisira07/PriceWatch/internal/models"
)

// fakeStore is an in-memory Store used purely for handler unit tests.
type fakeStore struct {
	usersByEmail  map[string]models.User
	items         map[string]models.WatchedItem
	history       map[string][]models.PriceHistory
	notifications map[string][]models.Notification
	nextID        int
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		usersByEmail:  map[string]models.User{},
		items:         map[string]models.WatchedItem{},
		history:       map[string][]models.PriceHistory{},
		notifications: map[string][]models.Notification{},
	}
}

func (f *fakeStore) genID() string {
	f.nextID++
	return "id-" + time.Now().Format("150405") + "-" + string(rune('a'+f.nextID))
}

func (f *fakeStore) CreateUser(_ context.Context, email, hash string) (models.User, error) {
	if _, exists := f.usersByEmail[email]; exists {
		return models.User{}, ErrDuplicateEmail
	}
	u := models.User{ID: f.genID(), Email: email, PasswordHash: hash, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.usersByEmail[email] = u
	return u, nil
}

func (f *fakeStore) GetUserByEmail(_ context.Context, email string) (models.User, error) {
	u, ok := f.usersByEmail[email]
	if !ok {
		return models.User{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeStore) CreateWatchedItem(_ context.Context, userID string, req models.CreateWatchedItemRequest) (models.WatchedItem, error) {
	it := models.WatchedItem{
		ID: f.genID(), UserID: userID, Name: req.Name, URL: req.URL,
		TargetPrice: req.TargetPrice, IsActive: true,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	f.items[it.ID] = it
	return it, nil
}

func (f *fakeStore) ListWatchedItems(_ context.Context, userID string) ([]models.WatchedItem, error) {
	var out []models.WatchedItem
	for _, it := range f.items {
		if it.UserID == userID {
			out = append(out, it)
		}
	}
	return out, nil
}

func (f *fakeStore) GetWatchedItem(_ context.Context, userID, itemID string) (models.WatchedItem, error) {
	it, ok := f.items[itemID]
	if !ok || it.UserID != userID {
		return models.WatchedItem{}, ErrNotFound
	}
	return it, nil
}

func (f *fakeStore) UpdateWatchedItem(_ context.Context, userID, itemID string, req models.UpdateWatchedItemRequest) (models.WatchedItem, error) {
	it, ok := f.items[itemID]
	if !ok || it.UserID != userID {
		return models.WatchedItem{}, ErrNotFound
	}
	if req.Name != nil {
		it.Name = *req.Name
	}
	if req.URL != nil {
		it.URL = *req.URL
	}
	if req.TargetPrice != nil {
		it.TargetPrice = *req.TargetPrice
	}
	if req.IsActive != nil {
		it.IsActive = *req.IsActive
	}
	f.items[itemID] = it
	return it, nil
}

func (f *fakeStore) DeleteWatchedItem(_ context.Context, userID, itemID string) error {
	it, ok := f.items[itemID]
	if !ok || it.UserID != userID {
		return ErrNotFound
	}
	delete(f.items, itemID)
	return nil
}

func (f *fakeStore) ListPriceHistory(_ context.Context, userID, itemID string) ([]models.PriceHistory, error) {
	it, ok := f.items[itemID]
	if !ok || it.UserID != userID {
		return nil, ErrNotFound
	}
	return f.history[it.ID], nil
}

func (f *fakeStore) ListNotifications(_ context.Context, userID, itemID string) ([]models.Notification, error) {
	it, ok := f.items[itemID]
	if !ok || it.UserID != userID {
		return nil, ErrNotFound
	}
	return f.notifications[it.ID], nil
}

func testAPI() (*API, *fakeStore, *auth.TokenIssuer) {
	store := newFakeStore()
	issuer := auth.NewTokenIssuer("test-secret", time.Hour)
	return New(store, issuer, nil), store, issuer
}

func doRequest(t *testing.T, h http.Handler, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestRegisterHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       models.RegisterRequest
		wantStatus int
	}{
		{"valid registration", models.RegisterRequest{Email: "a@example.com", Password: "supersecret1"}, http.StatusCreated},
		{"invalid email", models.RegisterRequest{Email: "not-an-email", Password: "supersecret1"}, http.StatusBadRequest},
		{"password too short", models.RegisterRequest{Email: "b@example.com", Password: "short"}, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, _, _ := testAPI()
			rr := doRequest(t, http.HandlerFunc(api.Register), http.MethodPost, "/auth/register", "", tt.body)
			if rr.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d, body=%s", tt.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}

	t.Run("duplicate email rejected", func(t *testing.T) {
		api, _, _ := testAPI()
		body := models.RegisterRequest{Email: "dup@example.com", Password: "supersecret1"}
		rr1 := doRequest(t, http.HandlerFunc(api.Register), http.MethodPost, "/auth/register", "", body)
		if rr1.Code != http.StatusCreated {
			t.Fatalf("first registration should succeed, got %d", rr1.Code)
		}
		rr2 := doRequest(t, http.HandlerFunc(api.Register), http.MethodPost, "/auth/register", "", body)
		if rr2.Code != http.StatusConflict {
			t.Fatalf("expected 409 on duplicate email, got %d", rr2.Code)
		}
	})
}

func TestLoginHandler(t *testing.T) {
	api, store, _ := testAPI()
	hash, err := auth.HashPassword("correctpassword")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	store.usersByEmail["user@example.com"] = models.User{ID: "u1", Email: "user@example.com", PasswordHash: hash}

	tests := []struct {
		name       string
		body       models.LoginRequest
		wantStatus int
		wantToken  bool
	}{
		{"correct credentials", models.LoginRequest{Email: "user@example.com", Password: "correctpassword"}, http.StatusOK, true},
		{"wrong password", models.LoginRequest{Email: "user@example.com", Password: "wrongpassword"}, http.StatusUnauthorized, false},
		{"unknown email", models.LoginRequest{Email: "nobody@example.com", Password: "whatever1"}, http.StatusUnauthorized, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := doRequest(t, http.HandlerFunc(api.Login), http.MethodPost, "/auth/login", "", tt.body)
			if rr.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d, body=%s", tt.wantStatus, rr.Code, rr.Body.String())
			}
			if tt.wantToken {
				var resp models.LoginResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if resp.Token == "" {
					t.Fatal("expected non-empty token")
				}
			}
		})
	}
}

func TestItemsCRUD_RequiresAuth(t *testing.T) {
	api, _, _ := testAPI()
	routes := api.Routes()

	rr := doRequest(t, routes, http.MethodGet, "/items", "", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rr.Code)
	}
}

func TestCreateAndGetItem_WithAuth(t *testing.T) {
	api, _, issuer := testAPI()
	routes := api.Routes()

	token, err := issuer.IssueToken("user-1", "user@example.com")
	if err != nil {
		t.Fatalf("setup token: %v", err)
	}

	createBody := models.CreateWatchedItemRequest{Name: "Widget", URL: "https://example.com/widget", TargetPrice: 19.99}
	rr := doRequest(t, routes, http.MethodPost, "/items", token, createBody)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating item, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var created models.WatchedItem
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created item: %v", err)
	}
	if created.Name != "Widget" || created.UserID != "user-1" {
		t.Fatalf("unexpected created item: %+v", created)
	}

	rr2 := doRequest(t, routes, http.MethodGet, "/items/"+created.ID, token, nil)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 fetching item, got %d", rr2.Code)
	}

	// A different user must not be able to see this item.
	otherToken, _ := issuer.IssueToken("user-2", "other@example.com")
	rr3 := doRequest(t, routes, http.MethodGet, "/items/"+created.ID, otherToken, nil)
	if rr3.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for other user's item, got %d", rr3.Code)
	}
}
