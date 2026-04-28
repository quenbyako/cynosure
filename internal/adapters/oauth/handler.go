// Package oauth implements OAuth adapter.
package oauth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/quenbyako/core"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
)

type Handler struct {
	tracer         ports.ObserveStack
	internalClient *http.Client
	externalClient *http.Client
	// default scopes are using when there is no information about required
	// scopes for getting data from specificed endpoint.
	defaultScopes []string
}

var _ oauthhandler.Factory = (*Handler)(nil)

func (h *Handler) OAuthHandler() oauthhandler.PortWrapped { return oauthhandler.Wrap(h, h.tracer) }

type newParams struct {
	internalTransport http.RoundTripper
	externalTransport http.RoundTripper
	metrics           core.Metrics
}

type NewOption func(*newParams)

func WithObservability(m core.Metrics) NewOption {
	return func(p *newParams) { p.metrics = m }
}

func WithTransports(internal, external http.RoundTripper) NewOption {
	return func(p *newParams) {
		p.internalTransport = internal
		p.externalTransport = external
	}
}

func New(defaultScopes []string, opts ...NewOption) *Handler {
	params := newParams{
		internalTransport: http.DefaultTransport,
		externalTransport: http.DefaultTransport,
		metrics:           core.NoopMetrics(),
	}
	for _, opt := range opts {
		opt(&params)
	}

	tracer := ports.StackFromCore(params.metrics, pkgName)

	return &Handler{
		internalClient: &http.Client{
			Transport:     params.internalTransport,
			CheckRedirect: nil,
			Jar:           nil,
			Timeout:       time.Minute,
		},
		externalClient: &http.Client{
			Transport:     params.externalTransport,
			CheckRedirect: nil,
			Jar:           nil,
			Timeout:       time.Minute,
		},
		defaultScopes: defaultScopes,
		tracer:        tracer,
	}
}

// Exchange implements ports.OAuthHandler.
func (h *Handler) Exchange(
	ctx context.Context, config *oauth2.Config, code string, verifier []byte,
) (*oauth2.Token, error) {
	cfg := injectTransport(h.externalClient, config)

	token, err := cfg.Exchange(ctx, code, oauth2.SetAuthURLParam(
		"code_verifier", base64.RawURLEncoding.EncodeToString(verifier),
	))
	if err != nil {
		return nil, fmt.Errorf("exchanging token: %w", err)
	}

	return token, nil
}

// RefreshToken implements [oauthhandler.Port].
func (h *Handler) RefreshToken(
	ctx context.Context, config *oauth2.Config, token *oauth2.Token,
	opts ...oauthhandler.RefreshTokenOption,
) (*oauth2.Token, error) {
	if token == nil {
		return nil, errInternalValidation("invalid token")
	}

	if token.RefreshToken == "" {
		return nil, errInternalValidation("no refresh token available")
	}

	params := oauthhandler.RefreshTokenParams(opts...)

	client := h.externalClient
	if params.Internal() {
		client = h.internalClient
	}

	cfg := injectTransport(client, config)

	newToken, err := cfg.TokenSource(ctx, token).Token()
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}

	return newToken, nil
}
