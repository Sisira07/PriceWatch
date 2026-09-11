// Package models contains the shared data structures used across the API
// and worker services. These map directly to the Postgres tables.
package models

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type WatchedItem struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Name         string    `json:"name"`
	URL          string    `json:"url"`
	TargetPrice  float64   `json:"target_price"`
	CurrentPrice *float64  `json:"current_price,omitempty"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PriceHistory struct {
	ID            string    `json:"id"`
	WatchedItemID string    `json:"watched_item_id"`
	Price         float64   `json:"price"`
	CheckedAt     time.Time `json:"checked_at"`
}

type TickResult struct {
	Checked       int `json:"checked"`
	Notifications int `json:"notifications"`
}

type Notification struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	WatchedItemID string    `json:"watched_item_id"`
	Message       string    `json:"message"`
	PriceAtAlert  float64   `json:"price_at_alert"`
	IsRead        bool      `json:"is_read"`
	CreatedAt     time.Time `json:"created_at"`
}

// --- Request/response DTOs ---

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type CreateWatchedItemRequest struct {
	Name        string  `json:"name"`
	URL         string  `json:"url"`
	TargetPrice float64 `json:"target_price"`
}

type UpdateWatchedItemRequest struct {
	Name        *string  `json:"name,omitempty"`
	URL         *string  `json:"url,omitempty"`
	TargetPrice *float64 `json:"target_price,omitempty"`
	IsActive    *bool    `json:"is_active,omitempty"`
}
