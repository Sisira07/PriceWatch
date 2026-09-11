// Command api runs the PriceWatch REST API service.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sisira07/PriceWatch/internal/auth"
	"github.com/Sisira07/PriceWatch/internal/db"
	"github.com/Sisira07/PriceWatch/internal/handlers"
	"github.com/Sisira07/PriceWatch/internal/worker"
)

func mustEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if fallback == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return fallback
}

func main() {
	dsn := mustEnv("DATABASE_URL", "")
	jwtSecret := mustEnv("JWT_SECRET", "")
	addr := mustEnv("API_ADDR", ":8080")

	if err := db.RunMigrations(dsn); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.New(ctx, db.Config{DSN: dsn})
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	issuer := auth.NewTokenIssuer(jwtSecret, 24*time.Hour)
	store := handlers.NewPGStore(pool)

	// The API process gets its own Worker instance sharing the same pool,
	// used only by the manual "simulate a tick now" endpoint (Tick() is
	// called directly; Run()'s ticker loop is never started here — that's
	// the separate worker service's job).
	workerStore := worker.NewPGStore(pool)
	wk := worker.New(workerStore, time.Minute)

	api := handlers.New(store, issuer, wk)

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("api: listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("api: server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("api: shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("api: shutdown error: %v", err)
	}
}
