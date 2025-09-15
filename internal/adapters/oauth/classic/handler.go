package oauth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"golang.org/x/oauth2"

	"tg-helper/internal/domains/ports"
)

type Handler struct {
	client *http.Client

	serviceName string
	authScopes  []string

	tracer trace.Tracer
}

var _ ports.OAuthHandler = (*Handler)(nil)

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

// RegisterClient implements ports.OAuthHandler.
func (h *Handler) RegisterClient(ctx context.Context, u *url.URL, clientName string, redirect *url.URL) (cfg *oauth2.Config, expiresAt time.Time, err error) {
	ctx, span := h.tracer.Start(ctx, "Handler.RegisterClient")
	defer span.End()

	metadata, err := h.getServerMetadata(ctx, u)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to get server metadata: %w", err)
	}

	if metadata.RegistrationEndpoint == "" {
		return nil, time.Time{}, errors.New("server does not support dynamic client registration")
	}

	registrationEndpoint, err := url.Parse(metadata.RegistrationEndpoint)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to parse registration endpoint: %w", err)
	}

	resp, err := fetchRegisterClient(ctx, h.client, registrationEndpoint, &registerClientRequestBody{
		ClientName:              clientName,
		RedirectURIs:            []string{redirect.String()},
		TokenEndpointAuthMethod: "client_secret_basic",
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		Scope:                   strings.Join(h.authScopes, " "),
	})
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("registering client: %w", err)
	}

	if resp.ExpiresAt > 0 {
		expiresAt = time.Unix(resp.ExpiresAt, 0)
	}

	return &oauth2.Config{
		ClientID:     resp.ClientID,
		ClientSecret: resp.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:       metadata.AuthorizationEndpoint,
			DeviceAuthURL: "",
			TokenURL:      metadata.TokenEndpoint,
			AuthStyle:     oauth2.AuthStyleAutoDetect,
		},
		RedirectURL: redirect.String(),
		Scopes:      h.authScopes,
	}, expiresAt, nil
}

func (h *Handler) getServerMetadata(ctx context.Context, u *url.URL) (*serverMetadataResponse, error) {
	// Try to discover the authorization server via OAuth Protected Resource
	// as per RFC 9728 (https://datatracker.ietf.org/doc/html/rfc9728)

	// Try to fetch the OAuth Protected Resource metadata
	protectedResourceURL := &url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
		Path:   "/.well-known/oauth-protected-resource",
	}

	authServerScheme, authServerHost := u.Scheme, u.Host

	protectedResource, err := fetchProtectedResource(ctx, h.client, protectedResourceURL, nil)
	if err == nil && len(protectedResource.AuthorizationServers) > 0 {
		authServer, err := url.Parse(protectedResource.AuthorizationServers[0])
		if err != nil {
			return nil, fmt.Errorf("received invalid authorization server URL %q: %w", protectedResource.AuthorizationServers[0], err)
		}

		authServerScheme, authServerHost = authServer.Scheme, authServer.Host
	} else if errors.Is(err, errNotFound) {
		// using default endpoint then
	} else if err != nil {
		return nil, fmt.Errorf("failed to fetch protected resource metadata: %w", err)
	}

	// Try OpenID Connect discovery first
	metadata, err := fetchMetadataFromURL(ctx, h.client, &url.URL{
		Scheme: authServerScheme,
		Host:   authServerHost,
		Path:   "/.well-known/openid-configuration",
	}, nil)
	if err == nil {
		return metadata, nil
	}

	// If OpenID Connect discovery fails, try OAuth Authorization Server Metadata
	metadata, err = fetchMetadataFromURL(ctx, h.client, &url.URL{
		Scheme: authServerScheme,
		Host:   authServerHost,
		Path:   "/.well-known/oauth-authorization-server",
	}, nil)
	if metadata != nil {
		return metadata, nil
	}

	// If both discovery methods fail, use default endpoints based on the authorization server URL
	metadata, err = getDefaultEndpoints(&url.URL{Scheme: authServerScheme, Host: authServerHost})
	if err != nil {
		return nil, fmt.Errorf("failed to get default endpoints: %w", err)
	}

	return metadata, nil
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
