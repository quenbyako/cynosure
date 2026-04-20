// Package oauth implements OAuth adapter.
package oauth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/quenbyako/core"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
)

type Handler struct {
	tracer ports.ObserveStack
	client *http.Client
	// default scopes are using when there is no information about required
	// scopes for getting data from specificed endpoint.
	defaultScopes []string
}

var _ oauthhandler.Factory = (*Handler)(nil)

func (h *Handler) OAuthHandler() oauthhandler.PortWrapped { return oauthhandler.Wrap(h, h.tracer) }

type newParams struct {
	client  http.RoundTripper
	metrics core.Metrics
}

type NewOption func(*newParams)

func WithObservability(m core.Metrics) NewOption {
	return func(p *newParams) { p.metrics = m }
}

func New(defaultScopes []string, opts ...NewOption) *Handler {
	params := newParams{
		client:  http.DefaultTransport,
		metrics: core.NoopMetrics(),
	}
	for _, opt := range opts {
		opt(&params)
	}

	tracer := ports.StackFromCore(params.metrics, pkgName)

	return &Handler{
		client: &http.Client{
			Transport: otelhttp.NewTransport(
				params.client,
				otelhttp.WithTracerProvider(params.metrics),
			),
			CheckRedirect: nil,
			Jar:           nil,
			Timeout:       0,
		},
		defaultScopes: defaultScopes,
		tracer:        tracer,
	}
}

// Exchange implements ports.OAuthHandler.
func (h *Handler) Exchange(
	ctx context.Context, config *oauth2.Config, code string, verifier []byte,
) (*oauth2.Token, error) {
	token, err := config.Exchange(ctx, code, oauth2.SetAuthURLParam(
		"code_verifier", base64.RawURLEncoding.EncodeToString(verifier),
	))
	if err != nil {
		return nil, fmt.Errorf("exchanging token: %w", err)
	}

	return token, nil
}

// RefreshToken implements ports.OAuthHandler.
func (h *Handler) RefreshToken(
	ctx context.Context, config *oauth2.Config, token *oauth2.Token,
) (*oauth2.Token, error) {
	if token == nil {
		return nil, errInternalValidation("invalid token")
	}

	if token.RefreshToken == "" {
		return nil, errInternalValidation("no refresh token available")
	}

	newToken, err := config.TokenSource(ctx, token).Token()
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}

	return newToken, nil
}
