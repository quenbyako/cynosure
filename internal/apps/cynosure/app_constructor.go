package cynosure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/quenbyako/core"
	"google.golang.org/grpc"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

var (
	ErrOryAdminKey     = errors.New("missing oryAdminKey")
	ErrOryEndpoint     = errors.New("missing oryEndpoint")
	ErrOryClientID     = errors.New("missing oryClientID")
	ErrOryClientSecret = errors.New("missing oryClientSecret")
	ErrOryScopes       = errors.New("missing oryScopes")
	ErrOryRedirectURL  = errors.New("missing oryRedirectURL")
	ErrTelegramKey     = errors.New("missing telegramKey")
	ErrTelegramPublic  = errors.New("missing telegramPublicAddr")
	ErrGeminiKey       = errors.New("missing geminiKey")
	ErrDatabaseURL     = errors.New("missing database URL")
	ErrAdminMCPID      = errors.New("missing adminMCPID")
)

type SecretGetter interface {
	Get(ctx context.Context) ([]byte, error)
}

type (
	appParams struct {
		ory      oryParams
		telegram telegramParams
		gemini   geminiParams
		storage  storageParams

		observability      core.Metrics
		grpcAddr           grpc.ServiceRegistrar
		httpAddr           func(http.Handler)
		mcpAddr            func(http.Handler)
		constructionErrors []error
		adminMCPID         ids.ServerID
	}

	oryParams struct {
		endpoint     *url.URL
		adminKey     SecretGetter
		clientID     string
		clientSecret SecretGetter
		redirectURL  string
		scopes       []string
		callback     *url.URL
		oauthScopes  []string
	}

	telegramParams struct {
		key        SecretGetter
		publicAddr *url.URL
		addr       func(http.Handler)
	}

	geminiParams struct {
		key SecretGetter
	}

	storageParams struct {
		databaseURL *url.URL
	}
)

func (p *appParams) validate() error {
	if len(p.constructionErrors) > 0 {
		return errors.Join(p.constructionErrors...)
	}

	return errors.Join(
		p.validateOry(),
		p.validateTelegram(),
		p.validateGemini(),
		p.validateStorage(),
		p.validateInfra(),
	)
}

func (p *appParams) validateOry() error {
	var errs []error
	if p.ory.adminKey == nil {
		errs = append(errs, ErrOryAdminKey)
	}

	if p.ory.endpoint == nil || p.ory.endpoint.Scheme == "" {
		errs = append(errs, ErrOryEndpoint)
	}

	if p.ory.clientID == "" {
		errs = append(errs, ErrOryClientID)
	}

	if p.ory.clientSecret == nil {
		errs = append(errs, ErrOryClientSecret)
	}

	if len(p.ory.scopes) == 0 {
		errs = append(errs, ErrOryScopes)
	}

	if p.ory.redirectURL == "" {
		errs = append(errs, ErrOryRedirectURL)
	}

	return errors.Join(errs...)
}

func (p *appParams) validateTelegram() error {
	var errs []error
	if p.telegram.key == nil {
		errs = append(errs, ErrTelegramKey)
	}

	if p.telegram.publicAddr == nil || p.telegram.publicAddr.Scheme == "" {
		errs = append(errs, ErrTelegramPublic)
	}

	return errors.Join(errs...)
}

func (p *appParams) validateGemini() error {
	if p.gemini.key == nil {
		return ErrGeminiKey
	}

	return nil
}

func (p *appParams) validateStorage() error {
	if p.storage.databaseURL == nil || p.storage.databaseURL.Scheme == "" {
		return ErrDatabaseURL
	}

	return nil
}

func (p *appParams) validateInfra() error {
	if !p.adminMCPID.Valid() {
		return ErrAdminMCPID
	}

	return nil
}

type AppOpts func(*appParams)

func WithGRPCServer(port grpc.ServiceRegistrar) AppOpts {
	return func(p *appParams) { p.grpcAddr = port }
}

func WithHTTPServer(registrar func(http.Handler)) AppOpts {
	return func(p *appParams) { p.httpAddr = registrar }
}

func WithTelegramServer(registrar func(http.Handler)) AppOpts {
	return func(p *appParams) { p.telegram.addr = registrar }
}

func WithTelegramPublicAddr(addr *url.URL) AppOpts {
	return func(p *appParams) { p.telegram.publicAddr = addr }
}

func WithTelegramKey(key SecretGetter) AppOpts {
	return func(p *appParams) { p.telegram.key = key }
}

func WithGeminiKey(key SecretGetter) AppOpts {
	return func(p *appParams) { p.gemini.key = key }
}

func WithObservability(metrics core.Metrics) AppOpts {
	return func(p *appParams) { p.observability = metrics }
}

func WithDatabaseURL(addr *url.URL) AppOpts {
	return func(p *appParams) { p.storage.databaseURL = addr }
}

func WithOry(endpoint *url.URL, adminKey SecretGetter) AppOpts {
	return func(p *appParams) {
		p.ory.endpoint = endpoint
		p.ory.adminKey = adminKey
	}
}

func WithOryClientCredentials(clientID string, clientSecret SecretGetter) AppOpts {
	return func(p *appParams) {
		p.ory.clientID = clientID
		p.ory.clientSecret = clientSecret
	}
}

func WithOryScopes(scopes ...string) AppOpts {
	return func(p *appParams) { p.ory.scopes = scopes }
}

func WithOryRedirectURL(oryRedirectURL string) AppOpts {
	return func(p *appParams) { p.ory.redirectURL = oryRedirectURL }
}

func WithOAuthCallbackURL(u *url.URL) AppOpts {
	return func(p *appParams) { p.ory.callback = u }
}

func WithMCP(registrar func(http.Handler)) AppOpts {
	return func(p *appParams) { p.mcpAddr = registrar }
}

func WithAdminMCPID(id string) AppOpts {
	return func(p *appParams) {
		var err error

		p.adminMCPID, err = ids.NewServerIDFromString(id)
		if err != nil {
			p.constructionErrors = append(p.constructionErrors, err)
		}
	}
}

func defaultOryParams() oryParams {
	callbackURL, err := url.Parse("http://localhost:5002/oauth/callback")
	if err != nil {
		panic("invalid default oauth callback url") //nolint:forbidigo // safe for constant
	}

	return oryParams{
		endpoint:     nil,
		adminKey:     nil,
		clientID:     "",
		clientSecret: nil,
		redirectURL:  "http://localhost:5001",
		scopes:       []string{"mcp:read", "mcp:write", "offline_access"},
		callback:     callbackURL,
		oauthScopes:  []string{"mcp.read", "mcp.write"},
	}
}

func defaultParams() appParams {
	return appParams{
		ory: defaultOryParams(),
		telegram: telegramParams{
			key:        nil,
			publicAddr: nil,
			addr:       nil,
		},
		gemini: geminiParams{
			key: nil,
		},
		storage: storageParams{
			databaseURL: nil,
		},
		observability:      core.NoopMetrics(),
		grpcAddr:           nil,
		httpAddr:           nil,
		mcpAddr:            nil,
		constructionErrors: nil,
		adminMCPID:         ids.ServerID{},
	}
}

func Build(ctx context.Context, opts ...AppOpts) (*App, error) {
	params := defaultParams()

	for _, opt := range opts {
		opt(&params)
	}

	if err := params.validate(); err != nil {
		return nil, fmt.Errorf("validating params: %w", err)
	}

	return buildApp(ctx, &params)
}

func connectDependencies(
	params *appParams,
	_ adminControllerWireBind,
	_ oauthControllerWireBind,
	_ telegramControllerWireBind,
	_ mcpControllerWireBind,
) (*App, error) {
	return &App{}, nil
}
