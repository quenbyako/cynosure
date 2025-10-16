package cynosure

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/quenbyako/cynosure/contrib/onelog"
	"go.opentelemetry.io/otel/metric"
	noopMetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	noopTrace "go.opentelemetry.io/otel/trace/noop"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"github.com/quenbyako/cynosure/internal/controllers/a2a"
	"github.com/quenbyako/cynosure/internal/controllers/admin"
	"github.com/quenbyako/cynosure/internal/controllers/oauth"
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

	log   slog.Handler
	trace trace.TracerProvider
	meter metric.MeterProvider

	storagePath   string
	oauthScopes   []string
	oauthCallback *url.URL
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

func WithObservability(
	log slog.Handler,
	meter metric.MeterProvider,
	tracer trace.TracerProvider,
) AppOpts {
	return func(p *appParams) {
		p.log = log
		p.meter = meter
		p.trace = tracer
	}
}

func WithDefaultModelConfig(modelID string) AppOpts {
	return func(p *appParams) { p.defaultModelConfig = modelID }
}

func NewApp(ctx context.Context, opts ...AppOpts) *App {
	p := appParams{
		log:   slog.DiscardHandler,
		trace: noopTrace.NewTracerProvider(),
		meter: noopMetric.NewMeterProvider(),

		storagePath:   "./data.yaml",
		oauthScopes:   []string{"mcp.read", "mcp.write"},
		oauthCallback: must(url.Parse("http://localhost:8080/oauth/callback")),
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
	a2a.Register(chat)(p.grpcAddr)
	admin.Register(accounts, servers)(p.grpcAddr)

	// http controllers
	p.httpAddr(oauth.NewHandler(accounts))

	return &App{
		log: onelog.Wrap(p.log),
	}, nil
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
