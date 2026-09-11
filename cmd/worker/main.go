// Command worker runs the PriceWatch background price-checking service.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Sisira07/PriceWatch/internal/db"
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
	intervalSec, err := strconv.Atoi(mustEnv("WORKER_INTERVAL_SECONDS", "60"))
	if err != nil {
		log.Fatalf("invalid WORKER_INTERVAL_SECONDS: %v", err)
	}

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

	store := worker.NewPGStore(pool)
	w := worker.New(store, time.Duration(intervalSec)*time.Second)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-stop
		log.Println("worker: received shutdown signal")
		cancel()
	}()

	w.Run(ctx) // blocks until ctx is cancelled
	log.Println("worker: stopped")
}