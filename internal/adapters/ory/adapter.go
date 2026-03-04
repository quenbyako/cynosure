package ory

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/quenbyako/core"
	"github.com/quenbyako/cynosure/contrib/ory-openapi/gen/go/ory"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
)

const pkgName = "github.com/quenbyako/cynosure/internal/adapters/ory"

type Client struct {
	baseURL  string
	adminKey string

	// For IssueToken
	config oauth2.Config

	obs   *observable
	trace ports.ObserveStack

	api *ory.ClientWithResponses
}

var _ identitymanager.PortFactory = (*Client)(nil)

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

func WithRedirectURL(url string) NewOption {
	return func(p *newParams) { p.redirectURL = url }
}

func New(endpoint *url.URL, adminKey string, opts ...NewOption) *Client {
	p := newParams{
		metrics: core.NoopMetrics(),
	}

	for _, opt := range opts {
		opt(&p)
	}

	// TODO: validate config
	conf := oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:   endpoint.String() + "/oauth2/auth",
			TokenURL:  endpoint.String() + "/oauth2/token",
			AuthStyle: oauth2.AuthStyleInHeader,
		},
		RedirectURL: p.redirectURL,
		Scopes:      p.scopes,
	}

	c := &Client{
		baseURL:  endpoint.String(),
		adminKey: adminKey,
		config:   conf,
		obs:      newObservable(ports.StackFromCore(p.metrics, pkgName)),
		trace:    ports.StackFromCore(p.metrics, pkgName),
	}

	apiClient, err := ory.NewClientWithResponses(c.baseURL, ory.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+c.adminKey)
		return nil
	}))
	if err != nil {
		panic(fmt.Errorf("creating ory api client: %w", err))
	}
	c.api = apiClient

	if err := c.validate(); err != nil {
		panic(err)
	}

	return c
}

func (a *Client) Valid() bool { return a != nil && a.validate() == nil }

func (a *Client) validate() error {
	if a.baseURL == "" {
		return fmt.Errorf("base url is required")
	}
	if a.adminKey == "" {
		return fmt.Errorf("admin key is required")
	}
	if err := validateOauthConfig(a.config); err != nil {
		return err
	}

	return nil
}

func validateOauthConfig(conf oauth2.Config) error {
	if conf.ClientID == "" {
		return fmt.Errorf("client id is required")
	}
	// client secrets are usually optional.

	if conf.Endpoint.AuthURL == "" {
		return fmt.Errorf("auth url is required")
	}
	if conf.Endpoint.TokenURL == "" {
		return fmt.Errorf("token url is required")
	}
	if conf.RedirectURL == "" {
		return fmt.Errorf("redirect url is required")
	}
	if len(conf.Scopes) == 0 {
		return fmt.Errorf("scopes are required")
	}
	return nil
}
