// Package handlers implements the HTTP layer of the API service: auth
// endpoints and full CRUD on watched items, plus price history lookup.
//
// All DB access goes through the Store interface so handlers can be unit
// tested against an in-memory fake instead of a real Postgres instance.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Sisira07/PriceWatch/internal/auth"
	"github.com/Sisira07/PriceWatch/internal/models"
	"github.com/Sisira07/PriceWatch/internal/worker"
)

var ErrNotFound = errors.New("not found")
var ErrDuplicateEmail = errors.New("email already registered")

// Store is the persistence contract handlers depend on. The real
// implementation wraps a pgx pool; tests use an in-memory fake.
type Store interface {
	CreateUser(ctx context.Context, email, passwordHash string) (models.User, error)
	GetUserByEmail(ctx context.Context, email string) (models.User, error)

	CreateWatchedItem(ctx context.Context, userID string, req models.CreateWatchedItemRequest) (models.WatchedItem, error)
	ListWatchedItems(ctx context.Context, userID string) ([]models.WatchedItem, error)
	GetWatchedItem(ctx context.Context, userID, itemID string) (models.WatchedItem, error)
	UpdateWatchedItem(ctx context.Context, userID, itemID string, req models.UpdateWatchedItemRequest) (models.WatchedItem, error)
	DeleteWatchedItem(ctx context.Context, userID, itemID string) error

	ListPriceHistory(ctx context.Context, userID, itemID string) ([]models.PriceHistory, error)
	ListNotifications(ctx context.Context, userID, itemID string) ([]models.Notification, error)
}

type API struct {
	Store  Store
	Issuer *auth.TokenIssuer
	// Worker is optional. When set, it powers the "simulate a tick now"
	// admin endpoint used by the demo frontend. Tests and any caller that
	// doesn't need that endpoint can pass nil.
	Worker *worker.Worker
}

func New(store Store, issuer *auth.TokenIssuer, wk *worker.Worker) *API {
	return &API{Store: store, Issuer: issuer, Worker: wk}
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}



// --- auth endpoints ---

func (a *API) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := auth.ValidateEmail(req.Email); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := a.Store.CreateUser(r.Context(), req.Email, hash)
	if err != nil {
		if errors.Is(err, ErrDuplicateEmail) {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create user")
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (a *API) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := a.Store.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err := auth.CheckPassword(user.PasswordHash, req.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := a.Issuer.IssueToken(user.ID, user.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue token")
		return
	}
	writeJSON(w, http.StatusOK, models.LoginResponse{Token: token})
}

// --- items CRUD ---

func (a *API) CreateItem(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())

	var req models.CreateWatchedItemRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.URL == "" || req.TargetPrice <= 0 {
		writeError(w, http.StatusBadRequest, "name, url and a positive target_price are required")
		return
	}

	item, err := a.Store.CreateWatchedItem(r.Context(), userID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create item")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) ListItems(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	items, err := a.Store.ListWatchedItems(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list items")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *API) GetItem(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	id := r.PathValue("id")

	item, err := a.Store.GetWatchedItem(r.Context(), userID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "item not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not fetch item")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) UpdateItem(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	id := r.PathValue("id")

	var req models.UpdateWatchedItemRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	item, err := a.Store.UpdateWatchedItem(r.Context(), userID, id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "item not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not update item")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) DeleteItem(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	id := r.PathValue("id")

	if err := a.Store.DeleteWatchedItem(r.Context(), userID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "item not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not delete item")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) GetItemHistory(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	id := r.PathValue("id")

	history, err := a.Store.ListPriceHistory(r.Context(), userID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "item not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not fetch history")
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func (a *API) GetItemNotifications(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	id := r.PathValue("id")

	notifications, err := a.Store.ListNotifications(r.Context(), userID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "item not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not fetch notifications")
		return
	}
	writeJSON(w, http.StatusOK, notifications)
}

// --- admin / demo endpoints ---

// SimulateTick runs one worker price-check pass immediately, on demand,
// instead of waiting for the worker's normal interval. It exists purely so
// the demo frontend can show cause-and-effect without a real wait — it does
// exactly what the worker's scheduled tick does, just triggered manually.
func (a *API) SimulateTick(w http.ResponseWriter, r *http.Request) {
	if a.Worker == nil {
		writeError(w, http.StatusNotImplemented, "simulate endpoint is not configured on this server")
		return
	}

	result, err := a.Worker.Tick(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "tick failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Routes wires up the mux for the API service.
func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /auth/register", a.Register)
	mux.HandleFunc("POST /auth/login", a.Login)

	protected := http.NewServeMux()
	protected.HandleFunc("POST /items", a.CreateItem)
	protected.HandleFunc("GET /items", a.ListItems)
	protected.HandleFunc("GET /items/{id}", a.GetItem)
	protected.HandleFunc("PATCH /items/{id}", a.UpdateItem)
	protected.HandleFunc("DELETE /items/{id}", a.DeleteItem)
	protected.HandleFunc("GET /items/{id}/history", a.GetItemHistory)
	protected.HandleFunc("GET /items/{id}/notifications", a.GetItemNotifications)
	protected.HandleFunc("POST /admin/simulate-tick", a.SimulateTick)

	mux.Handle("/items", RequireAuth(a.Issuer)(protected))
	mux.Handle("/items/", RequireAuth(a.Issuer)(protected))
	mux.Handle("/admin/", RequireAuth(a.Issuer)(protected))

	return cors(withTimeout(mux, 10*time.Second))
}

func withTimeout(h http.Handler, d time.Duration) http.Handler {
	return http.TimeoutHandler(h, d, `{"error":"request timed out"}`)
}
