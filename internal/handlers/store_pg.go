package handlers

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sisira07/PriceWatch/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore is the production Store implementation backed by Postgres via pgx.
type PGStore struct {
	Pool *pgxpool.Pool
}

func NewPGStore(pool *pgxpool.Pool) *PGStore {
	return &PGStore{Pool: pool}
}

const uniqueViolationCode = "23505"

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode
}

func (s *PGStore) CreateUser(ctx context.Context, email, passwordHash string) (models.User, error) {
	const q = `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash, created_at, updated_at`
	var u models.User
	err := s.Pool.QueryRow(ctx, q, email, passwordHash).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return models.User{}, ErrDuplicateEmail
		}
		return models.User{}, fmt.Errorf("insert user: %w", err)
	}
	return u, nil
}

func (s *PGStore) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	const q = `
		SELECT id, email, password_hash, created_at, updated_at
		FROM users WHERE email = $1`
	var u models.User
	err := s.Pool.QueryRow(ctx, q, email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, ErrNotFound
		}
		return models.User{}, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

func (s *PGStore) CreateWatchedItem(ctx context.Context, userID string, req models.CreateWatchedItemRequest) (models.WatchedItem, error) {
	const q = `
		INSERT INTO watched_items (user_id, name, url, target_price)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, name, url, target_price, current_price, is_active, created_at, updated_at`
	var it models.WatchedItem
	err := s.Pool.QueryRow(ctx, q, userID, req.Name, req.URL, req.TargetPrice).
		Scan(&it.ID, &it.UserID, &it.Name, &it.URL, &it.TargetPrice, &it.CurrentPrice, &it.IsActive, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		return models.WatchedItem{}, fmt.Errorf("insert item: %w", err)
	}
	return it, nil
}

func (s *PGStore) ListWatchedItems(ctx context.Context, userID string) ([]models.WatchedItem, error) {
	const q = `
		SELECT id, user_id, name, url, target_price, current_price, is_active, created_at, updated_at
		FROM watched_items WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := s.Pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
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

func (s *PGStore) GetWatchedItem(ctx context.Context, userID, itemID string) (models.WatchedItem, error) {
	const q = `
		SELECT id, user_id, name, url, target_price, current_price, is_active, created_at, updated_at
		FROM watched_items WHERE id = $1 AND user_id = $2`
	var it models.WatchedItem
	err := s.Pool.QueryRow(ctx, q, itemID, userID).
		Scan(&it.ID, &it.UserID, &it.Name, &it.URL, &it.TargetPrice, &it.CurrentPrice, &it.IsActive, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.WatchedItem{}, ErrNotFound
		}
		return models.WatchedItem{}, fmt.Errorf("get item: %w", err)
	}
	return it, nil
}

func (s *PGStore) UpdateWatchedItem(ctx context.Context, userID, itemID string, req models.UpdateWatchedItemRequest) (models.WatchedItem, error) {
	// Fetch-then-write keeps this readable as plain SQL instead of a
	// dynamic query builder; fine at this scale.
	current, err := s.GetWatchedItem(ctx, userID, itemID)
	if err != nil {
		return models.WatchedItem{}, err
	}
	if req.Name != nil {
		current.Name = *req.Name
	}
	if req.URL != nil {
		current.URL = *req.URL
	}
	if req.TargetPrice != nil {
		current.TargetPrice = *req.TargetPrice
	}
	if req.IsActive != nil {
		current.IsActive = *req.IsActive
	}

	const q = `
		UPDATE watched_items
		SET name = $1, url = $2, target_price = $3, is_active = $4, updated_at = now()
		WHERE id = $5 AND user_id = $6
		RETURNING id, user_id, name, url, target_price, current_price, is_active, created_at, updated_at`
	var it models.WatchedItem
	err = s.Pool.QueryRow(ctx, q, current.Name, current.URL, current.TargetPrice, current.IsActive, itemID, userID).
		Scan(&it.ID, &it.UserID, &it.Name, &it.URL, &it.TargetPrice, &it.CurrentPrice, &it.IsActive, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.WatchedItem{}, ErrNotFound
		}
		return models.WatchedItem{}, fmt.Errorf("update item: %w", err)
	}
	return it, nil
}

func (s *PGStore) DeleteWatchedItem(ctx context.Context, userID, itemID string) error {
	const q = `DELETE FROM watched_items WHERE id = $1 AND user_id = $2`
	tag, err := s.Pool.Exec(ctx, q, itemID, userID)
	if err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) ListPriceHistory(ctx context.Context, userID, itemID string) ([]models.PriceHistory, error) {
	// Confirm the item belongs to this user before returning history.
	if _, err := s.GetWatchedItem(ctx, userID, itemID); err != nil {
		return nil, err
	}

	const q = `
		SELECT id, watched_item_id, price, checked_at
		FROM price_history WHERE watched_item_id = $1 ORDER BY checked_at DESC`
	rows, err := s.Pool.Query(ctx, q, itemID)
	if err != nil {
		return nil, fmt.Errorf("list history: %w", err)
	}
	defer rows.Close()

	var history []models.PriceHistory
	for rows.Next() {
		var ph models.PriceHistory
		if err := rows.Scan(&ph.ID, &ph.WatchedItemID, &ph.Price, &ph.CheckedAt); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		history = append(history, ph)
	}
	return history, rows.Err()
}

func (s *PGStore) ListNotifications(ctx context.Context, userID, itemID string) ([]models.Notification, error) {
	// Confirm the item belongs to this user before returning its notifications.
	if _, err := s.GetWatchedItem(ctx, userID, itemID); err != nil {
		return nil, err
	}

	const q = `
		SELECT id, user_id, watched_item_id, message, price_at_alert, is_read, created_at
		FROM notifications WHERE watched_item_id = $1 ORDER BY created_at DESC`
	rows, err := s.Pool.Query(ctx, q, itemID)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var notifications []models.Notification
	for rows.Next() {
		var n models.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.WatchedItemID, &n.Message, &n.PriceAtAlert, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		notifications = append(notifications, n)
	}
	return notifications, rows.Err()
}
