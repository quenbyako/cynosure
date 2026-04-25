// Package ory provides an adapter for Ory Hydra and Kratos.
package ory

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/quenbyako/core"
	"github.com/quenbyako/cynosure/contrib/ory-openapi/gen/go/ory"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
)

const (
	pkgName = "github.com/quenbyako/cynosure/internal/adapters/ory"
)

type Adapter struct {
	// For IssueToken
	config    *injectedConfig
	transport http.RoundTripper
	trace     ports.ObserveStack
	obs       *observable
	api       *ory.ClientWithResponses
	baseURL   string
	adminKey  string
}

var _ identitymanager.PortFactory = (*Adapter)(nil)

// IdentityManager returns the identity manager port.
func (a *Adapter) IdentityManager() identitymanager.PortWrapped {
	return identitymanager.Wrap(a, a.trace)
}

type newParams struct {
	metrics      core.Metrics
	transport    http.RoundTripper
	clientID     string
	clientSecret string
	redirectURL  string
	scopes       []string
}

func buildNewParams(opts ...NewOption) newParams {
	params := newParams{
		metrics:      core.NoopMetrics(),
		clientID:     "",
		clientSecret: "",
		redirectURL:  "",
		scopes:       nil,
		transport:    http.DefaultTransport,
	}

	for _, opt := range opts {
		opt(&params)
	}

	return params
}

type NewOption func(*newParams)

func WithObservability(metrics core.Metrics) NewOption {
	return func(p *newParams) { p.metrics = metrics }
}

func WithClientCredentials(clientID, clientSecret string) NewOption {
	return func(p *newParams) {
		p.clientID = clientID
		p.clientSecret = clientSecret
	}
}

func WithScopes(scopes ...string) NewOption {
	return func(p *newParams) { p.scopes = scopes }
}

func WithRedirectURL(redirectURL string) NewOption {
	return func(p *newParams) { p.redirectURL = redirectURL }
}

func WithHTTPClient(client http.RoundTripper) NewOption {
	return func(p *newParams) { p.transport = client }
}

func New(endpoint *url.URL, adminKey string, opts ...NewOption) (*Adapter, error) {
	params := buildNewParams(opts...)

	config, err := newOauthConfig(endpoint.String(), params, params.transport)
	if err != nil {
		return nil, fmt.Errorf("new oauth config: %w", err)
	}

	client := &Adapter{
		baseURL:   endpoint.String(),
		adminKey:  adminKey,
		config:    config,
		transport: params.transport,
		obs:       newObservable(ports.StackFromCore(params.metrics, pkgName)),
		trace:     ports.StackFromCore(params.metrics, pkgName),
		api:       nil,
	}

	if err := client.initAPI(); err != nil {
		return nil, fmt.Errorf("init api: %w", err)
	}

	if !client.Valid() {
		return nil, fmt.Errorf("%w: invalid state", ErrInternal)
	}

	return client, nil
}

func (a *Adapter) initAPI() error {
	apiClient, err := ory.NewClientWithResponses(
		a.baseURL,
		ory.WithHTTPClient(&http.Client{
			Transport:     a.transport,
			CheckRedirect: nil,
			Jar:           nil,
			Timeout:       time.Minute, // TODO: configurable timeout for client?
		}),
		ory.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+a.adminKey)
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("creating ory api client: %w", err)
	}

	a.api = apiClient

	return nil
}

func newOauthConfig(
	baseURL string,
	params newParams,
	transport http.RoundTripper,
) (*injectedConfig, error) {
	cfg := oauth2.Config{
		ClientID:     params.clientID,
		ClientSecret: params.clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:       baseURL + "/oauth2/auth",
			TokenURL:      baseURL + "/oauth2/token",
			AuthStyle:     oauth2.AuthStyleInHeader,
			DeviceAuthURL: "",
		},
		RedirectURL: params.redirectURL,
		Scopes:      params.scopes,
	}

	if err := validateOauthConfig(cfg); err != nil {
		return nil, err
	}

	return injectTransport(&http.Client{
		Transport:     transport,
		CheckRedirect: nil,
		Jar:           nil,
		Timeout:       time.Minute,
	}, &cfg), nil
}

func (a *Adapter) Valid() bool { return a != nil && a.validate() == nil }

func (a *Adapter) validate() error {
	if a.baseURL == "" {
		return ErrBaseURLRequired
	}

	if a.adminKey == "" {
		return ErrAdminKeyRequired
	}

	return nil
}

//nolint:ireturn
func (a *Adapter) initiateAuth(ctx context.Context, name string) (context.Context, span) {
	return a.obs.initiateAuth(ctx, name)
}

func validateOauthConfig(conf oauth2.Config) error {
	if conf.ClientID == "" {
		return ErrClientIDRequired
	}

	if conf.Endpoint.AuthURL == "" {
		return ErrAuthURLRequired
	}

	if conf.Endpoint.TokenURL == "" {
		return ErrTokenURLRequired
	}

	if conf.RedirectURL == "" {
		return ErrRedirectURLRequired
	}

	if len(conf.Scopes) == 0 {
		return ErrScopesRequired
	}

	return nil
}
