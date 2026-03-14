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

const pkgName = "github.com/quenbyako/cynosure/internal/adapters/ory"

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

	// TODO: validate config
	conf := oauth2.Config{
		ClientID:     params.clientID,
		ClientSecret: params.clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:       endpoint.String() + "/oauth2/auth",
			TokenURL:      endpoint.String() + "/oauth2/token",
			AuthStyle:     oauth2.AuthStyleInHeader,
			DeviceAuthURL: "",
		},
		RedirectURL: params.redirectURL,
		Scopes:      params.scopes,
	}

	client := &Client{
		baseURL:  endpoint.String(),
		adminKey: adminKey,
		config:   conf,
		obs:      newObservable(ports.StackFromCore(params.metrics, pkgName)),
		trace:    ports.StackFromCore(params.metrics, pkgName),
		api:      nil,
	}

	apiClient, err := ory.NewClientWithResponses(client.baseURL, ory.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+client.adminKey)
		return nil
	}))
	if err != nil {
		panic(fmt.Errorf("creating ory api client: %w", err))
	}

	client.api = apiClient

	if err := client.validate(); err != nil {
		panic(err)
	}

	return client
}

func (a *Client) Valid() bool { return a != nil && a.validate() == nil }

func (a *Client) validate() error {
	if a.baseURL == "" {
		return errors.New("base url is required")
	}

	if a.adminKey == "" {
		return errors.New("admin key is required")
	}

	if err := validateOauthConfig(a.config); err != nil {
		return err
	}

	return nil
}

func validateOauthConfig(conf oauth2.Config) error {
	if conf.ClientID == "" {
		return errors.New("client id is required")
	}
	// client secrets are usually optional.

	if conf.Endpoint.AuthURL == "" {
		return errors.New("auth url is required")
	}

	if conf.Endpoint.TokenURL == "" {
		return errors.New("token url is required")
	}

	if conf.RedirectURL == "" {
		return errors.New("redirect url is required")
	}

	if len(conf.Scopes) == 0 {
		return errors.New("scopes are required")
	}

	return nil
}
