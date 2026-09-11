package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Sisira07/PriceWatch/internal/models"
)

func TestShouldNotify(t *testing.T) {
	tests := []struct {
		name         string
		currentPrice float64
		targetPrice  float64
		want         bool
	}{
		{"price above target: no notify", 25.00, 20.00, false},
		{"price equal to target: notify", 20.00, 20.00, true},
		{"price below target: notify", 15.00, 20.00, true},
		{"zero price below positive target: notify", 0, 5.00, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldNotify(tt.currentPrice, tt.targetPrice)
			if got != tt.want {
				t.Fatalf("shouldNotify(%v, %v) = %v, want %v", tt.currentPrice, tt.targetPrice, got, tt.want)
			}
		})
	}
}

func TestCheckPrice_DeterministicWithinSameHour(t *testing.T) {
	item := models.WatchedItem{ID: "item-1", TargetPrice: 100}
	p1 := checkPrice(item)
	p2 := checkPrice(item)
	if p1 != p2 {
		t.Fatalf("expected checkPrice to be deterministic within the same hour for the same item, got %v vs %v", p1, p2)
	}
	if p1 < 80 || p1 > 120 {
		t.Fatalf("expected mocked price within 80-120 for target 100, got %v", p1)
	}
}

func TestCheckPrice_DifferentItemsDiffer(t *testing.T) {
	a := models.WatchedItem{ID: "item-a", TargetPrice: 50}
	b := models.WatchedItem{ID: "item-b", TargetPrice: 50}
	if checkPrice(a) == checkPrice(b) {
		t.Log("warning: different items produced identical mocked prices (statistically possible but worth eyeballing)")
	}
}

// mockStore is a fully in-memory Store used to test Worker.Tick without a
// real database.
type mockStore struct {
	items                []models.WatchedItem
	recordedPrices       map[string]float64
	notifications        []struct{ userID, itemID string; price float64 }
	failListActive       bool
	failRecordForItemID  string
	failNotifyForItemID  string
}

func (m *mockStore) ListActiveWatchedItems(_ context.Context) ([]models.WatchedItem, error) {
	if m.failListActive {
		return nil, errors.New("boom")
	}
	return m.items, nil
}

func (m *mockStore) RecordPriceCheck(_ context.Context, itemID string, price float64) error {
	if itemID == m.failRecordForItemID {
		return errors.New("record failed")
	}
	if m.recordedPrices == nil {
		m.recordedPrices = map[string]float64{}
	}
	m.recordedPrices[itemID] = price

	// Mirror the real PGStore: persist the new current_price onto the item so
	// the next ListActiveWatchedItems() call reflects it, letting tests
	// exercise dedup behavior across multiple ticks.
	for i := range m.items {
		if m.items[i].ID == itemID {
			p := price
			m.items[i].CurrentPrice = &p
			break
		}
	}
	return nil
}

func (m *mockStore) CreateNotification(_ context.Context, userID, itemID string, price float64) error {
	if itemID == m.failNotifyForItemID {
		return errors.New("notify failed")
	}
	m.notifications = append(m.notifications, struct {
		userID, itemID string
		price           float64
	}{userID, itemID, price})
	return nil
}

func TestWorkerTick_NotifiesWhenPriceAtOrBelowTarget(t *testing.T) {
	// checkPrice returns 80%-120% of an item's own target price, so any
	// single item has roughly a 50% chance of qualifying on a given tick —
	// the target's magnitude doesn't change those odds. Use many
	// independently-seeded items (different IDs => different PRNG seeds) so
	// "zero notifications across all of them" is astronomically unlikely
	// (about 0.5^itemCount), instead of asserting on one coin flip.
	const itemCount = 25
	items := make([]models.WatchedItem, itemCount)
	for i := range items {
		items[i] = models.WatchedItem{
			ID: fmt.Sprintf("notify-test-item-%d", i), UserID: "user-1", TargetPrice: 100,
		}
	}
	store := &mockStore{items: items}
	w := New(store, time.Second)

	result, err := w.Tick(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ItemsChecked != itemCount {
		t.Fatalf("expected TickResult.ItemsChecked=%d, got %d", itemCount, result.ItemsChecked)
	}
	if result.NotificationsCreated == 0 {
		t.Fatal("expected TickResult.NotificationsCreated > 0 across many independent items")
	}
}

func TestWorkerTick_ContinuesAfterPerItemErrors(t *testing.T) {
	store := &mockStore{
		items: []models.WatchedItem{
			{ID: "item-fail-record", UserID: "user-1", TargetPrice: 1000000},
			{ID: "item-ok", UserID: "user-1", TargetPrice: 1000000},
		},
		failRecordForItemID: "item-fail-record",
	}
	w := New(store, time.Second)

	if _, err := w.Tick(context.Background()); err != nil {
		t.Fatalf("Tick should not bubble up per-item errors, got: %v", err)
	}

	if _, ok := store.recordedPrices["item-fail-record"]; ok {
		t.Fatal("expected no recorded price for the failing item")
	}
	if _, ok := store.recordedPrices["item-ok"]; !ok {
		t.Fatal("expected the second item to still be processed after the first item's failure")
	}
}

func TestWorkerTick_ListErrorPropagates(t *testing.T) {
	store := &mockStore{failListActive: true}
	w := New(store, time.Second)

	if _, err := w.Tick(context.Background()); err == nil {
		t.Fatal("expected an error when listing active items fails")
	}
}

func TestWorkerTick_NoActiveItems(t *testing.T) {
	store := &mockStore{items: nil}
	w := New(store, time.Second)

	if _, err := w.Tick(context.Background()); err != nil {
		t.Fatalf("unexpected error with no items: %v", err)
	}
	if len(store.notifications) != 0 {
		t.Fatal("expected no notifications when there are no items")
	}
}

func float64Ptr(f float64) *float64 { return &f }

func TestShouldCreateNotification(t *testing.T) {
	tests := []struct {
		name          string
		currentPrice  float64
		targetPrice   float64
		previousPrice *float64
		want          bool
	}{
		{"never checked before, qualifies now: notify", 90, 100, nil, true},
		{"never checked before, doesn't qualify: no notify", 110, 100, nil, false},
		{"previously above target, now qualifies: notify (new drop)", 90, 100, float64Ptr(110), true},
		{"previously already at/below target, still qualifies: no notify (dedup)", 85, 100, float64Ptr(95), false},
		{"previously at/below target, now recovered above: no notify", 105, 100, float64Ptr(95), false},
		{"previously above target, still above: no notify", 110, 100, float64Ptr(120), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldCreateNotification(tt.currentPrice, tt.targetPrice, tt.previousPrice)
			if got != tt.want {
				t.Fatalf("shouldCreateNotification(%v, %v, %v) = %v, want %v",
					tt.currentPrice, tt.targetPrice, tt.previousPrice, got, tt.want)
			}
		})
	}
}

func TestWorkerTick_NoDuplicateNotificationWhenAlreadyBelowTarget(t *testing.T) {
	// Whatever price checkPrice happens to produce this tick, an item that
	// was ALREADY at or below target on the previous tick must never
	// generate a new notification: either it's still low (dedup) or it
	// recovered above target (nothing to notify about either way).
	item := models.WatchedItem{
		ID: "item-1", UserID: "user-1", TargetPrice: 100,
		CurrentPrice: float64Ptr(90), // already below target as of the last check
	}
	store := &mockStore{items: []models.WatchedItem{item}}
	w := New(store, time.Second)

	result, err := w.Tick(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NotificationsCreated != 0 {
		t.Fatalf("expected no notifications for an item already below target, got %d", result.NotificationsCreated)
	}
}

func TestWorkerTick_SecondTickNeverAddsMoreNotificationsThanFirst(t *testing.T) {
	// checkPrice is deterministic per item within the same hour, so ticking
	// the same set of items twice in a row must never produce additional
	// notifications on the second tick: whatever each item's price/target
	// relationship was after tick 1 is exactly what it still is on tick 2.
	const itemCount = 20
	items := make([]models.WatchedItem, itemCount)
	for i := range items {
		items[i] = models.WatchedItem{
			ID: fmt.Sprintf("item-%d", i), UserID: "user-1", TargetPrice: 100,
		}
	}
	store := &mockStore{items: items}
	w := New(store, time.Second)

	firstResult, err := w.Tick(context.Background())
	if err != nil {
		t.Fatalf("tick 1: unexpected error: %v", err)
	}

	secondResult, err := w.Tick(context.Background())
	if err != nil {
		t.Fatalf("tick 2: unexpected error: %v", err)
	}

	if secondResult.NotificationsCreated != 0 {
		t.Fatalf("expected no new notifications on the second tick (dedup), tick 1 created %d, tick 2 created %d",
			firstResult.NotificationsCreated, secondResult.NotificationsCreated)
	}
}
