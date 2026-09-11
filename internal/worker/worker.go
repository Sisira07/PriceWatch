// Package worker implements the background price-checking loop.
//
// IMPORTANT: this project does not scrape real retail sites. checkPrice is a
// deterministic mock so the project is runnable/testable without hitting
// third-party services or violating anyone's terms of service. See README.md
// for the rationale.
package worker

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"math/rand"
	"time"

	"github.com/Sisira07/PriceWatch/internal/models"
)

// Store is the persistence contract the worker depends on. The real
// implementation wraps a pgx pool; tests use an in-memory fake.
type Store interface {
	ListActiveWatchedItems(ctx context.Context) ([]models.WatchedItem, error)
	RecordPriceCheck(ctx context.Context, itemID string, price float64) error
	CreateNotification(ctx context.Context, userID, itemID string, price float64) error
}

// Worker periodically checks prices for all active watched items.
type Worker struct {
	store    Store
	interval time.Duration
}

func New(store Store, interval time.Duration) *Worker {
	return &Worker{store: store, interval: interval}
}

// Run starts the ticker loop and blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf("worker: starting, interval=%s", w.interval)
	for {
		select {
		case <-ctx.Done():
			log.Println("worker: shutting down")
			return
		case <-ticker.C:
			if _, err := w.Tick(ctx); err != nil {
				log.Printf("worker: tick error: %v", err)
			}
		}
	}
}

// TickResult summarizes what a single Tick call did. Returned so callers —
// including the API's manual "simulate a tick now" endpoint — can report
// back something meaningful instead of a bare success/failure.
type TickResult struct {
	ItemsChecked         int `json:"items_checked"`
	NotificationsCreated int `json:"notifications_created"`
}

// Tick performs a single pass over all active watched items: mock-check each
// price, record it in price_history, and create a notification the moment
// the price transitions to at-or-below the user's target. If the price was
// already at-or-below target on a previous tick, no new notification is
// created — otherwise a user would get a fresh notification every tick for
// as long as the price stays low. A notification fires again only if the
// price rises back above target and then drops below it a second time.
func (w *Worker) Tick(ctx context.Context) (TickResult, error) {
	var result TickResult

	items, err := w.store.ListActiveWatchedItems(ctx)
	if err != nil {
		return result, fmt.Errorf("list active items: %w", err)
	}

	for _, item := range items {
		price := checkPrice(item)
		previousPrice := item.CurrentPrice // price as of before this tick, or nil if never checked

		if err := w.store.RecordPriceCheck(ctx, item.ID, price); err != nil {
			log.Printf("worker: failed to record price for item %s: %v", item.ID, err)
			continue
		}
		result.ItemsChecked++

		if shouldCreateNotification(price, item.TargetPrice, previousPrice) {
			if err := w.store.CreateNotification(ctx, item.UserID, item.ID, price); err != nil {
				log.Printf("worker: failed to create notification for item %s: %v", item.ID, err)
			} else {
				result.NotificationsCreated++
			}
		}
	}
	return result, nil
}

// shouldNotify is the core comparison rule: true when the observed price is
// at or below the user's target. shouldCreateNotification builds on this to
// additionally dedup repeat notifications for a price that's already low.
func shouldNotify(currentPrice, targetPrice float64) bool {
	return currentPrice <= targetPrice
}

// shouldCreateNotification decides whether this tick's price is a *new* drop
// worth notifying about: the price must be at or below target now, and it
// must not already have been at or below target on the previous tick (a nil
// previousPrice means this item has never been checked before, so any
// qualifying price counts as new).
func shouldCreateNotification(currentPrice, targetPrice float64, previousPrice *float64) bool {
	if !shouldNotify(currentPrice, targetPrice) {
		return false
	}
	if previousPrice == nil {
		return true
	}
	return !shouldNotify(*previousPrice, targetPrice)
}

// checkPrice is a MOCKED price source. It never makes a network call. It
// derives a pseudo-random but reproducible price by seeding a PRNG from the
// item's ID plus the current hour, so repeated ticks within the same hour
// vary a little while still being deterministic for a given item/hour pair
// (useful for tests and for demoing convergence toward a target price).
func checkPrice(item models.WatchedItem) float64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(item.ID))
	seed := int64(h.Sum64()) + time.Now().Unix()/3600

	r := rand.New(rand.NewSource(seed))

	// Base price drifts around the target: 80%-120% of target, so items
	// realistically cross the target threshold over time.
	factor := 0.8 + r.Float64()*0.4
	price := item.TargetPrice * factor

	// Round to 2 decimal places like real currency.
	return float64(int64(price*100+0.5)) / 100
}
