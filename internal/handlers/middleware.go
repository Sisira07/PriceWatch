package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/Sisira07/PriceWatch/internal/auth"
)

type ctxKey string

const userIDKey ctxKey = "user_id"

// RequireAuth validates the Authorization: Bearer <token> header and injects
// the authenticated user's ID into the request context.
func RequireAuth(issuer *auth.TokenIssuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
				writeError(w, http.StatusUnauthorized, "missing or malformed Authorization header")
				return
			}

			claims, err := issuer.VerifyToken(parts[1])
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func userIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok
}