package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type JWTAuthenticator struct {
	allowedIssuers map[string]struct{}
	logger         *slog.Logger

	jwks      map[string]*jose.JSONWebKeySet
	lastFetch map[string]time.Time
	mu        sync.RWMutex
}

func NewJWTAuthenticator(issuers []string, logger *slog.Logger) *JWTAuthenticator {
	if logger == nil {
		logger = slog.Default()
	}
	allowed := make(map[string]struct{})
	for _, iss := range issuers {
		allowed[strings.TrimRight(iss, "/")] = struct{}{}
	}
	return &JWTAuthenticator{
		allowedIssuers: allowed,
		logger:         logger,
		jwks:           make(map[string]*jose.JSONWebKeySet),
		lastFetch:      make(map[string]time.Time),
	}
}

func (a *JWTAuthenticator) WhoAmI(ctx context.Context, tokenStr string) (ids.UserID, error) {
	tok, err := jwt.ParseSigned(tokenStr, []jose.SignatureAlgorithm{jose.RS256, jose.ES256})
	if err != nil {
		return ids.UserID{}, fmt.Errorf("parsing jwt: %w", err)
	}

	if len(tok.Headers) > 0 {
		h := tok.Headers[0]
		a.logger.Debug("received token", "kid", h.KeyID, "alg", h.Algorithm)
	}

	// We need to peek at the claims to get the issuer before we can fetch the JWKS
	var unsafeClaims jwt.Claims
	if err := tok.UnsafeClaimsWithoutVerification(&unsafeClaims); err != nil {
		return ids.UserID{}, fmt.Errorf("peeking claims: %w", err)
	}

	iss := strings.TrimRight(unsafeClaims.Issuer, "/")
	if _, ok := a.allowedIssuers[iss]; !ok {
		return ids.UserID{}, fmt.Errorf("issuer %q is not allowed", unsafeClaims.Issuer)
	}

	issURL, err := url.Parse(iss)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("invalid issuer URL: %w", err)
	}
	jwksURL := issURL.JoinPath(".well-known/jwks.json")

	keySet, err := a.getKeySet(ctx, jwksURL)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("getting jwks for %q: %w", iss, err)
	}

	var claims jwt.Claims
	if err := tok.Claims(keySet, &claims); err != nil {
		return ids.UserID{}, fmt.Errorf("verifying signature: %w", err)
	}

	// Validate claims
	expected := jwt.Expected{
		Issuer: iss,
		Time:   time.Now(),
	}

	if err := claims.Validate(expected); err != nil {
		// Try with 5 minutes leeway for time-based claims
		leeway := 5 * time.Minute

		now := time.Now()
		if claims.Expiry != nil && now.After(claims.Expiry.Time().Add(leeway)) {
			a.logger.Warn("token expired", "expiry", claims.Expiry.Time(), "now", now)
		}

		if claims.NotBefore != nil && now.Before(claims.NotBefore.Time().Add(-leeway)) {
			a.logger.Warn("token not active yet", "nbf", claims.NotBefore.Time(), "now", now)
		}

		return ids.UserID{}, fmt.Errorf("validating claims: %w (expected iss: %s, actual: %s)", err, iss, claims.Issuer)
	}

	if claims.Subject == "" {
		return ids.UserID{}, errors.New("missing sub claim")
	}

	a.logger.Debug("token authenticated successfully", "sub", claims.Subject)

	return ids.NewUserIDFromString(claims.Subject)
}

func (a *JWTAuthenticator) getKeySet(ctx context.Context, jwksURL *url.URL) (*jose.JSONWebKeySet, error) {
	key := jwksURL.String()

	a.mu.RLock()
	ks, ok := a.jwks[key]
	t, tok := a.lastFetch[key]
	if ok && tok && time.Since(t) < time.Hour {
		a.mu.RUnlock()
		return ks, nil
	}
	a.mu.RUnlock()

	a.mu.Lock()
	defer a.mu.Unlock()

	// Double check after lock
	ks, ok = a.jwks[key]
	t, tok = a.lastFetch[key]
	if ok && tok && time.Since(t) < time.Hour {
		return ks, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, key, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch jwks (status %d)", resp.StatusCode)
	}

	var newJwks jose.JSONWebKeySet
	if err := json.NewDecoder(resp.Body).Decode(&newJwks); err != nil {
		return nil, err
	}

	a.jwks[key] = &newJwks
	a.lastFetch[key] = time.Now()

	return &newJwks, nil
}
