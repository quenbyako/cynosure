package cynosure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/quenbyako/core"
	"github.com/quenbyako/cynosure/contrib/core-params/ratelimit"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type SecretGetter interface {
	Get(ctx context.Context) ([]byte, error)
}

type (
	appParams struct {
		telegram           telegramParams
		gemini             geminiParams
		internalMcpClient  http.RoundTripper
		externalMcpClient  http.RoundTripper
		observability      core.Metrics
		grpcAddr           grpc.ServiceRegistrar
		storage            storageParams
		httpAddr           func(http.Handler)
		redis              redisParams
		mcpAddr            func(http.Handler)
		ory                oryParams
		constructionErrors []error
		chat               chatParams
		adminMCPID         ids.ServerID
		rate               rateParams
	}

	rateParams struct {
		chatInput       ratelimit.Policy
		chatOutput      ratelimit.Policy
		embedding       ratelimit.Policy
		chatInputGlobal ratelimit.Policy
		embeddingGlobal ratelimit.Policy
		maxWait         time.Duration
		defaultPlanID   uuid.UUID
	}

	oryParams struct {
		adminKey     SecretGetter
		clientSecret SecretGetter
		apiClient    http.RoundTripper
		endpoint     *url.URL
		callback     *url.URL
		clientID     string
		redirectURL  string
		scopes       []string
		oauthScopes  []string
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
		client redis.UniversalClient
	}
	chatParams struct {
		historyLimit uint
	}
)

func (p *appParams) validate(ctx context.Context) error {
	if len(p.constructionErrors) > 0 {
		return errors.Join(p.constructionErrors...)
	}

	return errors.Join(
		p.validateOry(),
		p.validateTelegram(),
		p.validateGemini(),
		p.validateStorage(),
		p.validateInfra(),
		p.rate.validate(),
		p.validateMCPClient(ctx),
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

func (p *rateParams) validate() error {
	if p.chatInput.Period() <= 0 || p.chatInput.Limit() <= 0 {
		return MissingParamError("chat input rate limit")
	}

	if p.chatOutput.Period() <= 0 || p.chatOutput.Limit() <= 0 {
		return MissingParamError("chat output rate limit")
	}

	if p.embedding.Period() <= 0 || p.embedding.Limit() <= 0 {
		return MissingParamError("embedding rate limit")
	}

	if p.chatInputGlobal.Period() <= 0 || p.chatInputGlobal.Limit() <= 0 {
		return MissingParamError("chat input global rate limit")
	}

	if p.embeddingGlobal.Period() <= 0 || p.embeddingGlobal.Limit() <= 0 {
		return MissingParamError("embedding global rate limit")
	}

	if p.maxWait <= 0 {
		return MissingParamError("max wait time limit")
	}

	return nil
}

func (p *appParams) validateMCPClient(_ context.Context) error {
	if p.externalMcpClient == nil {
		return MissingParamError("externalMcpClient")
	}

	if p.internalMcpClient == nil {
		return MissingParamError("internalMcpClient")
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

func WithChatInputRateLimit(limit ratelimit.Policy) AppOpts {
	return func(p *appParams) { p.rate.chatInput = limit }
}

func WithChatOutputRateLimit(limit ratelimit.Policy) AppOpts {
	return func(p *appParams) { p.rate.chatOutput = limit }
}

func WithEmbeddingRateLimit(limit ratelimit.Policy) AppOpts {
	return func(p *appParams) { p.rate.embedding = limit }
}

func WithGlobalChatInputRateLimit(limit ratelimit.Policy) AppOpts {
	return func(p *appParams) { p.rate.chatInputGlobal = limit }
}

func WithGlobalEmbeddingRateLimit(limit ratelimit.Policy) AppOpts {
	return func(p *appParams) { p.rate.embeddingGlobal = limit }
}

func WithMaxWaitTimeLimit(limit time.Duration) AppOpts {
	return func(p *appParams) { p.rate.maxWait = limit }
}

func WithDefaultPlanID(id uuid.UUID) AppOpts {
	return func(p *appParams) { p.rate.defaultPlanID = id }
}

func WithChatLimits(historyLimit uint) AppOpts {
	return func(p *appParams) { p.chat.historyLimit = historyLimit }
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

func WithMCPTransports(internal, external http.RoundTripper) AppOpts {
	return func(p *appParams) {
		p.internalMcpClient, p.externalMcpClient = internal, external
	}
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
		ory:                defaultOryParams(),
		telegram:           defaultTelegramParams(),
		gemini:             defaultGeminiParams(),
		storage:            defaultStorageParams(),
		redis:              defaultRedisParams(),
		chat:               defaultChatParams(),
		rate:               defaultRateParams(),
		observability:      core.NoopMetrics(),
		grpcAddr:           nil,
		httpAddr:           nil,
		mcpAddr:            nil,
		constructionErrors: nil,
		adminMCPID:         ids.ServerID{},
		internalMcpClient:  nil,
		externalMcpClient:  nil,
	}
}

func defaultTelegramParams() telegramParams {
	return telegramParams{
		key:        nil,
		publicAddr: nil,
		register:   func(h http.Handler) {},
		apiClient:  http.DefaultTransport,
	}
}

func defaultGeminiParams() geminiParams {
	return geminiParams{
		key:       nil,
		apiClient: http.DefaultTransport,
	}
}

func defaultStorageParams() storageParams {
	return storageParams{
		databaseURL: nil,
	}
}

func defaultRedisParams() redisParams {
	return redisParams{
		client: nil,
	}
}

func defaultChatParams() chatParams {
	return chatParams{
		historyLimit: DefaultHistoryLimit,
	}
}

func defaultRateParams() rateParams {
	return rateParams{
		chatInput:     ratelimit.Policy{},
		chatOutput:    ratelimit.Policy{},
		embedding:     ratelimit.Policy{},
		defaultPlanID: uuid.Nil,
	}
}

func Build(ctx context.Context, opts ...AppOpts) (*App, error) {
	params := defaultParams()

	for _, opt := range opts {
		opt(&params)
	}

	if err := params.validate(ctx); err != nil {
		return nil, fmt.Errorf("validating params: %w", err)
	}

	app, err := buildApp(ctx, &params)
	return app, err
}

//nolint:unparam // wire needs these parameters to be present to correctly bind dependencies
func connectDependencies(
	params *appParams,
	lifecycle *lifecycle,
	_ adminControllerWireBind,
	_ oauthControllerWireBind,
	_ telegramControllerWireBind,
	_ mcpControllerWireBind,
) (*App, error) {
	return &App{
		lifecycle: lifecycle,
	}, nil
}
