package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type contextKey struct{}

// FromContext extracts the UserID from the context if it exists.
func FromContext(ctx context.Context) (ids.UserID, bool) {
	if id, ok := ctx.Value(contextKey{}).(ids.UserID); ok {
		return id, true
	}

	return ids.UserID{}, false
}

// WithUserID decorates context with UserID.
func WithUserID(ctx context.Context, id ids.UserID) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// UserIDProvider defines the interface to get UserID from raw token.
type UserIDProvider interface {
	WhoAmI(ctx context.Context, token string) (ids.UserID, error)
}

// Middleware extracts the user ID from the Authorization: Bearer <token> header and puts it into the context.
func Middleware(issuers []string, logger *slog.Logger) func(http.Handler) http.Handler {
	auth := NewJWTAuthenticator(issuers, logger)

	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if h == "" {
				logger.Warn("missing Authorization header")
				http.Error(w, "missing Authorization header", http.StatusUnauthorized)
				return
			}

			if !strings.HasPrefix(h, "Bearer ") {
				logger.Warn("invalid Authorization header format")
				http.Error(w, "invalid Authorization header format, expected Bearer <token>", http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(h, "Bearer ")

			id, err := auth.WhoAmI(r.Context(), token)
			if err != nil {
				logger.Error("authentication failed", "error", err)
				http.Error(w, fmt.Sprintf("authentication failed: %v", err), http.StatusUnauthorized)
				return
			}

			ctx := WithUserID(r.Context(), id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
