package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"golang.org/x/net/idna"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const (
	jwksCacheTTL = time.Hour
)

type issuerCache struct {
	jwks      *jose.JSONWebKeySet
	lastFetch time.Time
}

type JWTAuthenticator struct {
	logger slog.Handler

	// list of keys is always static, so it's safe to use _, ok := a.allowedIssuers[iss].
	// Key is host, nothing more. URL must be built from issuer domain
	allowedIssuers map[string]issuerCache
	mu             sync.RWMutex
}

func NewJWTAuthenticator(issuers []string, logger slog.Handler) *JWTAuthenticator {
	allowed := make(map[string]issuerCache)

	for _, iss := range issuers {
		allowed[iss] = issuerCache{
			jwks:      nil,
			lastFetch: time.Time{},
		}
	}

	return &JWTAuthenticator{
		logger:         logger,
		allowedIssuers: allowed,
		mu:             sync.RWMutex{},
	}
}

func (a *JWTAuthenticator) WhoAmI(ctx context.Context, tokenStr string) (ids.UserID, error) {
	tok, err := jwt.ParseSigned(tokenStr, []jose.SignatureAlgorithm{jose.RS256, jose.ES256})
	if err != nil {
		return ids.UserID{}, fmt.Errorf("parsing jwt: %w", err)
	}

	a.logTokenHeaders(ctx, tok)

	issuerDomain, err := a.getIssuerDomain(tok)
	if err != nil {
		return ids.UserID{}, err
	}

	keySet, err := a.resolveKeySet(ctx, issuerDomain)
	if err != nil {
		return ids.UserID{}, err
	}

	var claims jwt.Claims

	claims, err = a.verifyToken(tok, keySet, issuerDomain)
	if err != nil {
		return ids.UserID{}, err
	}

	return a.extractUserID(ctx, claims)
}

func (a *JWTAuthenticator) logTokenHeaders(ctx context.Context, tok *jwt.JSONWebToken) {
	if len(tok.Headers) > 0 {
		h := tok.Headers[0]
		logDebug(ctx, a.logger, "received token", "kid", h.KeyID, "alg", h.Algorithm)
	}
}

func (a *JWTAuthenticator) getIssuerDomain(tok *jwt.JSONWebToken) (string, error) {
	var unsafeClaims jwt.Claims
	if err := tok.UnsafeClaimsWithoutVerification(&unsafeClaims); err != nil {
		return "", fmt.Errorf("peeking claims: %w", err)
	}

	iss, err := url.Parse(unsafeClaims.Issuer)
	if err != nil {
		return "", fmt.Errorf("invalid issuer URL: %w", err)
	}

	if _, ok := a.allowedIssuers[iss.Host]; !ok {
		return "", fmt.Errorf("%w: %q", ErrIssuerNotAllowed, unsafeClaims.Issuer)
	}

	return iss.Host, nil
}

func (a *JWTAuthenticator) resolveKeySet(
	ctx context.Context,
	issuerDomain string,
) (
	*jose.JSONWebKeySet,
	error,
) {
	keySet, err := a.getKeySet(ctx, issuerDomain)
	if err != nil {
		return nil, fmt.Errorf("getting jwks for %q: %w", issuerDomain, err)
	}

	return keySet, nil
}

func (a *JWTAuthenticator) verifyToken(
	tok *jwt.JSONWebToken,
	keySet *jose.JSONWebKeySet,
	iss string,
) (
	jwt.Claims,
	error,
) {
	var claims jwt.Claims
	if err := tok.Claims(keySet, &claims); err != nil {
		return jwt.Claims{}, fmt.Errorf("verifying signature: %w", err)
	}

	expected := jwt.Expected{
		Issuer:      iss,
		Subject:     "",
		ID:          "",
		Time:        time.Now(),
		AnyAudience: nil,
	}

	if err := claims.Validate(expected); err != nil {
		return jwt.Claims{}, fmt.Errorf(
			"validating claims: %w (expected iss: %s, actual: %s)",
			err, iss, claims.Issuer,
		)
	}

	return claims, nil
}

func (a *JWTAuthenticator) extractUserID(
	ctx context.Context,
	claims jwt.Claims,
) (
	ids.UserID,
	error,
) {
	if claims.Subject == "" {
		return ids.UserID{}, ErrMissingSubClaim
	}

	logDebug(ctx, a.logger, "token authenticated successfully", "sub", claims.Subject)

	uid, err := ids.NewUserIDFromString(claims.Subject)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("creating user id: %w", err)
	}

	return uid, nil
}

func (a *JWTAuthenticator) getKeySet(
	ctx context.Context,
	issuerDomain string,
) (
	*jose.JSONWebKeySet,
	error,
) {
	if ks, ok := a.getFromCache(issuerDomain); ok {
		return ks, nil
	}

	return a.fetchAndCacheKeySet(ctx, issuerDomain)
}

func (a *JWTAuthenticator) getFromCache(issuerDomain string) (*jose.JSONWebKeySet, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	keys, ok := a.allowedIssuers[issuerDomain]
	if !ok {
		return nil, false
	}

	if time.Since(keys.lastFetch) < jwksCacheTTL {
		return keys.jwks, true
	}

	return nil, false
}

func (a *JWTAuthenticator) fetchAndCacheKeySet(
	ctx context.Context,
	issuerDomain string,
) (
	*jose.JSONWebKeySet,
	error,
) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Double check after lock
	keys, ok := a.allowedIssuers[issuerDomain]
	if !ok {
		return nil, ErrIssuerNotAllowed
	}

	if time.Since(keys.lastFetch) < jwksCacheTTL {
		return keys.jwks, nil
	}

	newJwks, err := a.fetchKeySet(ctx, issuerDomain)
	if err != nil {
		return nil, err
	}

	a.allowedIssuers[issuerDomain] = issuerCache{
		jwks:      newJwks,
		lastFetch: time.Now(),
	}

	return newJwks, nil
}

func (a *JWTAuthenticator) fetchKeySet(
	ctx context.Context,
	issuerDomain string,
) (
	*jose.JSONWebKeySet,
	error,
) {
	req, err := buildJwksRequest(ctx, issuerDomain)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching jwks: %w", err)
	}
	//nolint:errcheck // closing body is enough
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w (status %d)", ErrJwksFetchStatus, resp.StatusCode)
	}

	var newJwks jose.JSONWebKeySet
	if err := json.NewDecoder(resp.Body).Decode(&newJwks); err != nil {
		return nil, fmt.Errorf("decoding jwks: %w", err)
	}

	return &newJwks, nil
}

func buildJwksRequest(ctx context.Context, issuerDomain string) (*http.Request, error) {
	// double check that we don't have any injections
	if !validateHostPort(issuerDomain) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidIssuerDomain, issuerDomain)
	}

	endpoint := fmt.Sprintf("https://%s/.well-known/jwks.json", issuerDomain)

	//nolint:gosec // false positive, domain is allowlisted through allowedIssuers map.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating jwks request: %w", err)
	}

	return req, nil
}

func validateHostPort(input string) bool {
	// plain ip?
	if net.ParseIP(input) != nil {
		return true
	}

	host, ok := trimPort(input)
	if !ok {
		return false
	}

	// ipv6?
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") &&
		net.ParseIP(host[1:len(host)-1]) != nil {
		return true
	}

	// host ip?
	if net.ParseIP(host) != nil {
		return true
	}

	// punycode?
	host, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return false
	}

	return validDomainSyntax(host)
}

func trimPort(input string) (host string, ok bool) {
	lastColon := strings.LastIndex(input, ":")
	if lastColon < 0 {
		return input, true
	}

	switch port, err := strconv.Atoi(input[lastColon+1:]); {
	case err != nil:
		// all input might be an IP address
		return input, true
	case port > 0 && port <= 65535:
		return input[:lastColon], true
	default:
		return "", false
	}
}

// validDomainSyntax checks if the domain name is valid.
func validDomainSyntax(domain string) bool {
	const (
		maxDomainLength = 253
		maxLabelLength  = 63
	)

	if domain == "" || len(domain) > maxDomainLength {
		return false
	}

	labels := strings.Split(domain, ".")
	for _, label := range labels {
		switch {
		case label == "",
			len(label) > maxLabelLength,
			label[0] == '-',
			label[len(label)-1] == '-',
			strings.ContainsFunc(label, invalidLabelChar):
			return false
		}
	}

	return true
}

func invalidLabelChar(char rune) bool {
	return (char < 'a' || char > 'z') &&
		(char < 'A' || char > 'Z') &&
		(char < '0' || char > '9') &&
		char != '-'
}
