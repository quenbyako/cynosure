package cynosure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	mcpraw "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/quenbyako/core"
	"google.golang.org/grpc"

	"github.com/quenbyako/cynosure/internal/controllers/admin"
	"github.com/quenbyako/cynosure/internal/controllers/mcp"
	"github.com/quenbyako/cynosure/internal/controllers/oauth"
	"github.com/quenbyako/cynosure/internal/controllers/telegram"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/accounts"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/users"
)

type SecretGetter interface {
	Get(ctx context.Context) ([]byte, error)
}

type appParams struct {
	geminiKey          SecretGetter
	telegramKey        SecretGetter
	telegramPublicAddr *url.URL
	defaultModelConfig string
	oryAdminKey        SecretGetter
	oryEndpoint        *url.URL

	grpcAddr grpc.ServiceRegistrar
	// TODO: join into one handler
	httpAddr     func(http.Handler)
	telegramAddr func(http.Handler)
	mcpAddr      func(http.Handler)

	observability core.Metrics

	databaseURL   *url.URL
	oauthScopes   []string
	oauthCallback *url.URL
	anonUser      ids.UserID
}

func (p *appParams) validate() error {
	var errs []error
	if p.geminiKey == nil {
		errs = append(errs, errors.New("missing geminiKey"))
	}
	if p.telegramKey == nil {
		errs = append(errs, errors.New("missing telegramKey"))
	}
	if p.telegramPublicAddr == nil || p.telegramPublicAddr.Scheme == "" {
		errs = append(errs, errors.New("missing telegramPublicAddr"))
	}
	if p.defaultModelConfig == "" {
		errs = append(errs, errors.New("missing defaultModelConfig"))
	} else if err := uuid.Validate(p.defaultModelConfig); err != nil {
		errs = append(errs, fmt.Errorf("invalid defaultModelConfig: %w", err))
	}
	if p.oryAdminKey == nil {
		errs = append(errs, errors.New("missing oryAdminKey"))
	}
	if p.oryEndpoint == nil || p.oryEndpoint.Scheme == "" {
		errs = append(errs, errors.New("missing oryEndpoint"))
	}

	if !p.anonUser.Valid() {
		errs = append(errs, errors.New("missing anonUser"))
	}
	if p.databaseURL == nil || p.databaseURL.Scheme == "" {
		errs = append(errs, errors.New("missing database URL"))
	}

	return errors.Join(errs...)
}

type AppOpts func(*appParams)

func WithGRPCServer(port grpc.ServiceRegistrar) AppOpts {
	return func(p *appParams) { p.grpcAddr = port }
}

func WithHTTPServer(registrar func(http.Handler)) AppOpts {
	return func(p *appParams) { p.httpAddr = registrar }
}

func WithTelegramServer(registrar func(http.Handler)) AppOpts {
	return func(p *appParams) { p.telegramAddr = registrar }
}

func WithTelegramPublicAddr(addr *url.URL) AppOpts {
	return func(p *appParams) { p.telegramPublicAddr = addr }
}

func WithTelegramKey(key SecretGetter) AppOpts {
	return func(p *appParams) { p.telegramKey = key }
}

func WithGeminiKey(key SecretGetter) AppOpts {
	return func(p *appParams) { p.geminiKey = key }
}

func WithObservability(metrics core.Metrics) AppOpts {
	return func(p *appParams) { p.observability = metrics }
}

func WithDefaultModelConfig(modelID string) AppOpts {
	return func(p *appParams) { p.defaultModelConfig = modelID }
}

func WithDatabaseURL(addr *url.URL) AppOpts {
	return func(p *appParams) { p.databaseURL = addr }
}

func WithOry(endpoint *url.URL, adminKey SecretGetter) AppOpts {
	return func(p *appParams) { p.oryEndpoint, p.oryAdminKey = endpoint, adminKey }
}

func WithMCP(registrar func(http.Handler)) AppOpts {
	return func(p *appParams) { p.mcpAddr = registrar }
}

func Build(ctx context.Context, opts ...AppOpts) *App {
	p := appParams{
		observability: core.NoopMetrics(),

		oauthScopes:   []string{"mcp.read", "mcp.write"},
		oauthCallback: must(url.Parse("http://localhost:5002/oauth/callback")),
		anonUser:      must(ids.NewUserIDFromString("ff06b500-0000-0000-0000-000000000001")),
	}
	for _, opt := range opts {
		opt(&p)
	}
	if err := p.validate(); err != nil {
		panic(err)
	}

	return must(buildApp(ctx, &p))
}

var mcpImpl = mcpraw.Implementation{
	Name:       "admin-mcp-server",
	Title:      "Admin MCP Server",
	Version:    "1.0.0",
	WebsiteURL: "https://t.me/zhopakotabot",
}

func connectDependencies(
	ctx context.Context,
	p *appParams,
	log telegram.LogCallbacks,
	chat *chat.Usecase,
	accounts *accounts.Usecase,
	users *users.Usecase,
) (*App, error) {
	telegramKey, err := p.telegramKey.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get telegram key: %w", err)
	}

	// grpc controllers
	admin.Register(accounts)(p.grpcAddr)

	// http controllers
	p.httpAddr(oauth.NewHandler(accounts))

	// TODO: each of controllers MUST be separated, like adapters and usecases.
	p.telegramAddr(must(telegram.New(ctx, chat, users, p.telegramPublicAddr, telegramKey, telegram.WithLogCallbacks(log), telegram.WithTracer(p.observability))))

	p.mcpAddr(mcp.New(accounts, mcpImpl, mcp.WithLogger(p.observability)))

	return &App{}, nil
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
