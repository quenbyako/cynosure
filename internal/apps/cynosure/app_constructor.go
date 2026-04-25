package cynosure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/quenbyako/core"
	"github.com/quenbyako/cynosure/contrib/core-params/ratelimit"
	"google.golang.org/grpc"

	"github.com/quenbyako/cynosure/internal/adapters/inmemory"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const (
	DefaultSoftLimit = 20
	DefaultHardCap   = 50
)

type SecretGetter interface {
	Get(ctx context.Context) ([]byte, error)
}

type (
	appParams struct {
		telegram           telegramParams
		observability      core.Metrics
		gemini             geminiParams
		grpcAddr           grpc.ServiceRegistrar
		redis              redisParams
		storage            storageParams
		httpAddr           func(http.Handler)
		mcpAddr            func(http.Handler)
		ory                oryParams
		constructionErrors []error
		chat               chatParams
		rateLimit          ratelimit.Policy
		adminMCPID         ids.ServerID
		mcpClient          http.RoundTripper
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
		apiClient    http.RoundTripper
	}

	telegramParams struct {
		key        SecretGetter
		publicAddr *url.URL
		register   func(http.Handler)
		apiClient  http.RoundTripper
	}

	geminiParams struct {
		key       SecretGetter
		apiClient http.RoundTripper
	}

	storageParams struct {
		databaseURL *url.URL
	}

	redisParams struct {
		url *url.URL
	}
	chatParams struct {
		softLimit uint
		hardCap   uint
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
		p.validateRateLimit(),
	)
}

func (p *appParams) validateOry() error {
	var errs []error
	if p.ory.adminKey == nil {
		errs = append(errs, MissingParamError("oryAdminKey"))
	}

	if p.ory.endpoint == nil || p.ory.endpoint.Scheme == "" {
		errs = append(errs, MissingParamError("oryEndpoint"))
	}

	if p.ory.clientID == "" {
		errs = append(errs, MissingParamError("oryClientID"))
	}

	if p.ory.clientSecret == nil {
		errs = append(errs, MissingParamError("oryClientSecret"))
	}

	if len(p.ory.scopes) == 0 {
		errs = append(errs, MissingParamError("oryScopes"))
	}

	if p.ory.redirectURL == "" {
		errs = append(errs, MissingParamError("oryRedirectURL"))
	}

	return errors.Join(errs...)
}

func (p *appParams) validateTelegram() error {
	var errs []error
	if p.telegram.key == nil {
		errs = append(errs, MissingParamError("telegramKey"))
	}

	if p.telegram.publicAddr == nil || p.telegram.publicAddr.Scheme == "" {
		errs = append(errs, MissingParamError("telegramPublicAddr"))
	}

	return errors.Join(errs...)
}

func (p *appParams) validateGemini() error {
	if p.gemini.key == nil {
		return MissingParamError("geminiKey")
	}

	return nil
}

func (p *appParams) validateStorage() error {
	if p.storage.databaseURL == nil || p.storage.databaseURL.Scheme == "" {
		return MissingParamError("database URL")
	}

	return nil
}

func (p *appParams) validateInfra() error {
	if !p.adminMCPID.Valid() {
		return MissingParamError("adminMCPID")
	}

	return nil
}

func (p *appParams) validateRateLimit() error {
	if p.rateLimit.Period() <= 0 || p.rateLimit.Burst() <= 0 {
		return MissingParamError("rate limit")
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
	return func(p *appParams) { p.telegram.register = registrar }
}

func WithTelegramClient(client http.RoundTripper) AppOpts {
	return func(p *appParams) { p.telegram.apiClient = client }
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

func WithGeminiClient(client http.RoundTripper) AppOpts {
	return func(p *appParams) { p.gemini.apiClient = client }
}

func WithObservability(metrics core.Metrics) AppOpts {
	return func(p *appParams) { p.observability = metrics }
}

func WithDatabaseURL(addr *url.URL) AppOpts {
	return func(p *appParams) { p.storage.databaseURL = addr }
}

func WithRedis(addr *url.URL) AppOpts {
	return func(p *appParams) { p.redis.url = addr }
}

func WithRateLimit(limit ratelimit.Policy) AppOpts {
	return func(p *appParams) { p.rateLimit = limit }
}

func WithChatLimits(softLimit, hardCap uint) AppOpts {
	return func(p *appParams) {
		p.chat.softLimit = softLimit
		p.chat.hardCap = hardCap
	}
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

func WithOryClient(client http.RoundTripper) AppOpts {
	return func(p *appParams) { p.ory.apiClient = client }
}

func WithMCP(registrar func(http.Handler)) AppOpts {
	return func(p *appParams) { p.mcpAddr = registrar }
}

func WithMCPClient(client http.RoundTripper) AppOpts {
	return func(p *appParams) { p.mcpClient = client }
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
		apiClient:    http.DefaultTransport,
	}
}

func defaultParams() appParams {
	return appParams{
		ory: defaultOryParams(),
		telegram: telegramParams{
			key:        nil,
			publicAddr: nil,
			register:   func(h http.Handler) {},
			apiClient:  http.DefaultTransport,
		},
		gemini: geminiParams{
			key:       nil,
			apiClient: http.DefaultTransport,
		},
		storage: storageParams{
			databaseURL: nil,
		},
		redis: redisParams{
			url: nil,
		},
		chat: chatParams{
			softLimit: DefaultSoftLimit,
			hardCap:   DefaultHardCap,
		},
		observability:      core.NoopMetrics(),
		grpcAddr:           nil,
		httpAddr:           nil,
		mcpAddr:            nil,
		constructionErrors: nil,
		adminMCPID:         ids.ServerID{},
		rateLimit:          ratelimit.Policy{},
		mcpClient:          http.DefaultTransport,
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

//nolint:unparam // wire needs these parameters to be present to correctly bind dependencies
func connectDependencies(
	params *appParams,
	ratelimiter *inmemory.RateLimiter,
	_ adminControllerWireBind,
	_ oauthControllerWireBind,
	_ telegramControllerWireBind,
	_ mcpControllerWireBind,
) (*App, error) {
	return &App{
		jobs: []func(context.Context) error{
			ratelimiter.Cleanup,
		},
	}, nil
}
