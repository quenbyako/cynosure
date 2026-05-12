// Package ory provides an adapter for Ory Hydra and Kratos.
package ory

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/quenbyako/core"
	"github.com/quenbyako/cynosure/contrib/ory-openapi/gen/go/ory"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const (
	pkgName = "github.com/quenbyako/cynosure/internal/adapters/ory"

	tgIDCacheSize = 1000
)

type Adapter struct {
	transport    http.RoundTripper
	observeStack ports.ObserveStack
	// For IssueToken
	config      *injectedConfig
	obs         *observable
	api         *ory.ClientWithResponses
	userIDCache *lru.Cache[string, ids.UserID]
}

var _ identitymanager.PortFactory = (*Adapter)(nil)

// IdentityManager returns the identity manager port.
func (a *Adapter) IdentityManager() identitymanager.PortWrapped {
	return identitymanager.Wrap(a, a.observeStack)
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

	cache, err := lru.New[string, ids.UserID](tgIDCacheSize)
	if err != nil {
		return nil, fmt.Errorf("new lru cache: %w", err)
	}

	api, err := newAPI(endpoint.String(), adminKey, params.transport)
	if err != nil {
		return nil, fmt.Errorf("init api: %w", err)
	}

	observeStack := ports.StackFromCore(params.metrics, pkgName)

	return buildAdapter(params, config, cache, api, observeStack)
}

func buildAdapter(
	params newParams,
	oauthConfig *injectedConfig,
	cache *lru.Cache[string, ids.UserID],
	api *ory.ClientWithResponses,
	observeStack ports.ObserveStack,
) (*Adapter, error) {
	adapter := &Adapter{
		config:       oauthConfig,
		transport:    params.transport,
		obs:          newObservable(observeStack),
		observeStack: observeStack,
		api:          api,
		userIDCache:  cache,
	}
	if err := adapter.validate(); err != nil {
		return nil, fmt.Errorf("failed to create adapter: %w", err)
	}

	return adapter, nil
}

func newAPI(baseURL, apiKey string, transport http.RoundTripper) (*ory.ClientWithResponses, error) {
	apiClient, err := ory.NewClientWithResponses(
		baseURL,
		ory.WithHTTPClient(&http.Client{
			Transport:     transport,
			CheckRedirect: nil,
			Jar:           nil,
			Timeout:       time.Minute, // TODO: configurable timeout for client?
		}),
		ory.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+apiKey)
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("creating ory api client: %w", err)
	}

	return apiClient, nil
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
	return nil
}

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
