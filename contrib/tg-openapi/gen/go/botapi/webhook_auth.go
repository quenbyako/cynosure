package botapi

import (
	"crypto/subtle"
	"net/http"
)

const (
	//nolint:gosec // false positive
	TokenHeader = "X-Telegram-Bot-Api-Secret-Token"
)

// AuthenticateWebhook returns a MiddlewareFunc that verifies the telegram
// token header.
func AuthenticateWebhook(secret string) MiddlewareFunc {
	secretBytes := []byte(secret)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := []byte(r.Header.Get(TokenHeader))
			if len(header) == 0 {
				http.Error(w, "Missing secret token", http.StatusUnauthorized)

				return
			}

			// using subtle to prevent timing attacks.
			if subtle.ConstantTimeCompare(secretBytes, header) == 0 {
				http.Error(w, "Invalid secret token", http.StatusForbidden)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
