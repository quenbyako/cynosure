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

// Middleware extracts the user ID from the Authorization: Bearer <token> header
// and puts it into the context.
func Middleware(issuers []string, logger slog.Handler) func(http.Handler) http.Handler {
	auth := NewJWTAuthenticator(issuers, logger)

	return func(next http.Handler) http.Handler {
		return &authMiddleware{
			auth:   auth,
			logger: logger,
			next:   next,
		}
	}
}

type authMiddleware struct {
	auth   UserIDProvider
	logger slog.Handler
	next   http.Handler
}

func (m *authMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token, ok := m.extractToken(w, r)
	if !ok {
		return
	}

	id, err := m.auth.WhoAmI(r.Context(), token)
	if err != nil {
		logError(r.Context(), m.logger, "authentication failed", "error", err)
		http.Error(
			w, fmt.Sprintf("authentication failed: %v", err),
			http.StatusUnauthorized,
		)

		return
	}

	ctx := WithUserID(r.Context(), id)
	m.next.ServeHTTP(w, r.WithContext(ctx))
}

func (m *authMiddleware) extractToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		logWarn(r.Context(), m.logger, "missing Authorization header")
		http.Error(w, "missing Authorization header", http.StatusUnauthorized)

		return "", false
	}

	if !strings.HasPrefix(header, "Bearer ") {
		logWarn(r.Context(), m.logger, "invalid Authorization header format")
		http.Error(
			w, "invalid Authorization header format, expected Bearer <token>",
			http.StatusUnauthorized,
		)

		return "", false
	}

	return strings.TrimPrefix(header, "Bearer "), true
}
