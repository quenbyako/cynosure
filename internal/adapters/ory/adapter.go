// Package ory provides an adapter for Ory Hydra and Kratos.
package ory

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/quenbyako/core"
	"github.com/quenbyako/cynosure/contrib/ory-openapi/gen/go/ory"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
)

const (
	pkgName = "github.com/quenbyako/cynosure/internal/adapters/ory"
)

// Package-level error variables.
var (
	ErrBaseURLRequired     = errors.New("base url is required")
	ErrAdminKeyRequired    = errors.New("admin key is required")
	ErrClientIDRequired    = errors.New("client id is required")
	ErrAuthURLRequired     = errors.New("auth url is required")
	ErrTokenURLRequired    = errors.New("token url is required")
	ErrRedirectURLRequired = errors.New("redirect url is required")
	ErrScopesRequired      = errors.New("scopes are required")
	ErrIdentityMissing     = errors.New("invalid response from ory: missing identity")
	ErrRedirectMissing     = errors.New("invalid response from ory: missing redirect_to")
	ErrTooManyRedirects    = errors.New("too many redirects")
	ErrRateLimited         = identitymanager.ErrRateLimited
	ErrInternal            = errors.New("internal ory adapter error")
	ErrUnexpectedResponse  = errors.New("unexpected response from ory")
)

type Client struct {
	// For IssueToken
	config   oauth2.Config
	trace    ports.ObserveStack
	obs      *observable
	api      *ory.ClientWithResponses
	baseURL  string
	adminKey string
}

var _ identitymanager.PortFactory = (*Client)(nil)

// IdentityManager returns the identity manager port.
//
//nolint:ireturn // Implementing interface from external package.
func (a *Client) IdentityManager() identitymanager.PortWrapped {
	return identitymanager.Wrap(a, a.trace)
}

type newParams struct {
	metrics      core.Metrics
	clientID     string
	clientSecret string
	redirectURL  string
	scopes       []string
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

func New(endpoint *url.URL, adminKey string, opts ...NewOption) *Client {
	params := newParams{
		metrics:      core.NoopMetrics(),
		clientID:     "",
		clientSecret: "",
		redirectURL:  "",
		scopes:       nil,
	}

	for _, opt := range opts {
		opt(&params)
	}

	client := &Client{
		baseURL:  endpoint.String(),
		adminKey: adminKey,
		config:   newOauthConfig(endpoint.String(), params),
		obs:      newObservable(ports.StackFromCore(params.metrics, pkgName)),
		trace:    ports.StackFromCore(params.metrics, pkgName),
		api:      nil,
	}

	if err := client.initAPI(); err != nil {
		return nil
	}

	if !client.Valid() {
		return nil
	}

	return client
}

func (a *Client) initAPI() error {
	apiClient, err := ory.NewClientWithResponses(
		a.baseURL,
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

func newOauthConfig(baseURL string, params newParams) oauth2.Config {
	return oauth2.Config{
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
}

func (a *Client) Valid() bool { return a != nil && a.validate() == nil }

func (a *Client) validate() error {
	if a.baseURL == "" {
		return ErrBaseURLRequired
	}

	if a.adminKey == "" {
		return ErrAdminKeyRequired
	}

	if err := validateOauthConfig(a.config); err != nil {
		return err
	}

	return nil
}

//nolint:ireturn
func (a *Client) initiateAuth(ctx context.Context, name string) (context.Context, span) {
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
