package oauth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

type Handler struct {
	client *http.Client

	serviceName string
	authScopes  []string

	tracer trace.Tracer
}

var _ ports.OAuthHandlerFactory = (*Handler)(nil)

func (h *Handler) OAuthHandler() ports.OAuthHandler { return h }

type newParams struct {
	client http.RoundTripper
	tracer trace.TracerProvider
}

type NewOption func(*newParams)

func WithTracerProvider(tp trace.TracerProvider) NewOption {
	return func(p *newParams) { p.tracer = tp }
}

func New(scopes []string, opts ...NewOption) *Handler {
	p := newParams{
		client: http.DefaultTransport,
		tracer: noop.NewTracerProvider(),
	}
	for _, opt := range opts {
		opt(&p)
	}

	return &Handler{
		client:     &http.Client{Transport: otelhttp.NewTransport(p.client, otelhttp.WithTracerProvider(p.tracer))},
		authScopes: scopes,
		tracer:     p.tracer.Tracer(pkgName),
	}
}

// Exchange implements ports.OAuthHandler.
func (h *Handler) Exchange(ctx context.Context, config *oauth2.Config, code string, verifier []byte) (*oauth2.Token, error) {
	ctx, span := h.tracer.Start(ctx, "Handler.Exchange")
	defer span.End()

	return config.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", base64.RawURLEncoding.EncodeToString(verifier)))
}

// RefreshToken implements ports.OAuthHandler.
func (h *Handler) RefreshToken(ctx context.Context, config *oauth2.Config, token *oauth2.Token) (*oauth2.Token, error) {
	ctx, span := h.tracer.Start(ctx, "Handler.RefreshToken")
	defer span.End()

	if token == nil {
		return nil, errors.New("invalid token")
	}

	if token.RefreshToken == "" {
		return nil, errors.New("no refresh token available")
	}

	newToken, err := config.TokenSource(ctx, token).Token()
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}

	return newToken, nil
}

// getDefaultEndpoints returns default OAuth endpoints based on the base URL
func getDefaultEndpoints(u *url.URL) (*serverMetadataResponse, error) {
	// Validate that the URL has a scheme and host
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid base URL: missing scheme or host in %q", u)
	}

	return &serverMetadataResponse{
		Issuer:                (&url.URL{Scheme: u.Scheme, Host: u.Host}).String(),
		AuthorizationEndpoint: (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/authorize"}).String(),
		TokenEndpoint:         (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/token"}).String(),
		RegistrationEndpoint:  (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/register"}).String(),
	}, nil
}
