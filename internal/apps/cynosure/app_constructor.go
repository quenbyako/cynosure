package cynosure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/quenbyako/core"
	"github.com/quenbyako/cynosure/contrib/onelog"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"github.com/quenbyako/cynosure/internal/controllers/a2a"
	"github.com/quenbyako/cynosure/internal/controllers/admin"
	"github.com/quenbyako/cynosure/internal/controllers/oauth"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/accounts"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/servers"
)

type SecretGetter interface {
	Get(ctx context.Context) ([]byte, error)
}

type appParams struct {
	geminiKey          SecretGetter
	zepKey             SecretGetter
	defaultModelConfig string

	grpcAddr grpc.ServiceRegistrar
	httpAddr func(http.Handler)

	observability core.Metrics

	storagePath   string
	oauthScopes   []string
	oauthCallback *url.URL
	anonUser      ids.UserID
}

func (p *appParams) validate() error {
	var errs []error
	if p.zepKey == nil {
		errs = append(errs, errors.New("missing zepKey"))
	}
	if p.geminiKey == nil {
		errs = append(errs, errors.New("missing geminiKey"))
	}
	if p.defaultModelConfig == "" {
		errs = append(errs, errors.New("missing defaultModelConfig"))
	} else if err := uuid.Validate(p.defaultModelConfig); err != nil {
		errs = append(errs, fmt.Errorf("invalid defaultModelConfig: %w", err))
	}
	if !p.anonUser.Valid() {
		errs = append(errs, errors.New("missing anonUser"))
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

func WithZepKey(key SecretGetter) AppOpts {
	return func(p *appParams) { p.zepKey = key }
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

func NewApp(ctx context.Context, opts ...AppOpts) *App {
	p := appParams{
		observability: core.NoopMetrics(),

		storagePath:   "./data.yaml",
		oauthScopes:   []string{"mcp.read", "mcp.write"},
		oauthCallback: must(url.Parse("http://localhost:8080/oauth/callback")),
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

func newApp(
	p *appParams,
	chat *chat.Service,
	accounts *accounts.Service,
	servers *servers.Service,
) (*App, error) {
	// grpc controllers
	a2a.Register(chat, p.anonUser)(p.grpcAddr)
	admin.Register(accounts, servers)(p.grpcAddr)

	// http controllers
	p.httpAddr(oauth.NewHandler(accounts))

	return &App{
		log: onelog.Wrap(p.observability),
	}, nil
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
