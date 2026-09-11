package worker

import (
	"context"
	"fmt"

	"github.com/Sisira07/PriceWatch/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore is the production Store implementation backed by Postgres via pgx.
type PGStore struct {
	Pool *pgxpool.Pool
}

func NewPGStore(pool *pgxpool.Pool) *PGStore {
	return &PGStore{Pool: pool}
}

func (s *PGStore) ListActiveWatchedItems(ctx context.Context) ([]models.WatchedItem, error) {
	const q = `
		SELECT id, user_id, name, url, target_price, current_price, is_active, created_at, updated_at
		FROM watched_items WHERE is_active = true`
	rows, err := s.Pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list active items: %w", err)
	}
	defer rows.Close()

	var items []models.WatchedItem
	for rows.Next() {
		var it models.WatchedItem
		if err := rows.Scan(&it.ID, &it.UserID, &it.Name, &it.URL, &it.TargetPrice, &it.CurrentPrice, &it.IsActive, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *PGStore) RecordPriceCheck(ctx context.Context, itemID string, price float64) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op if committed

	if _, err := tx.Exec(ctx,
		`INSERT INTO price_history (watched_item_id, price) VALUES ($1, $2)`,
		itemID, price,
	); err != nil {
		return fmt.Errorf("insert price_history: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE watched_items SET current_price = $1, updated_at = now() WHERE id = $2`,
		price, itemID,
	); err != nil {
		return fmt.Errorf("update current_price: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *PGStore) CreateNotification(ctx context.Context, userID, itemID string, price float64) error {
	const q = `
		INSERT INTO notifications (user_id, watched_item_id, message, price_at_alert)
		VALUES ($1, $2, $3, $4)`
	msg := fmt.Sprintf("Price dropped to %.2f, which meets your target for this item.", price)
	_, err := s.Pool.Exec(ctx, q, userID, itemID, msg, price)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}