package tghelper

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"

	"tg-helper/contrib/onelog"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"tg-helper/internal/adapters/account-storage/file"
	"tg-helper/internal/adapters/gemini"
	modelConfigs "tg-helper/internal/adapters/model-settings/file"
	oauthClient "tg-helper/internal/adapters/oauth/classic"
	serversFile "tg-helper/internal/adapters/server-storage/file"
	primitive "tg-helper/internal/adapters/tool-handler"
	"tg-helper/internal/adapters/zep"
	"tg-helper/internal/apps/tghelper/metrics"
	"tg-helper/internal/controllers/a2a"
	"tg-helper/internal/controllers/admin"
	"tg-helper/internal/controllers/oauth"
	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/services/accounts"
	"tg-helper/internal/domains/services/chat"
	"tg-helper/internal/domains/services/servers"
)

type appParams struct {
	zepKey             string
	traceEndpoint      string
	defaultModelConfig string

	log         slog.Handler
	grpcAddr    net.Addr
	httpAddr    net.Addr
	metricsAddr net.Addr
}

func (p *appParams) validate() error {
	var errs []error
	if p.zepKey == "" {
		errs = append(errs, errors.New("missing zepKey"))
	}
	if p.traceEndpoint == "" {
		errs = append(errs, errors.New("missing traceEndpoint"))
	}
	if p.defaultModelConfig == "" {
		errs = append(errs, errors.New("missing defaultModelConfig"))
	} else if err := uuid.Validate(p.defaultModelConfig); err != nil {
		errs = append(errs, fmt.Errorf("invalid defaultModelConfig: %w", err))
	}

	return errors.Join(errs...)
}

type AppOpts func(*appParams)

func WithLog(log slog.Handler) AppOpts {
	return func(p *appParams) { p.log = log }
}

func WithGRPCPort(port uint16) AppOpts {
	return func(p *appParams) { p.grpcAddr = &net.TCPAddr{Port: int(port)} }
}

func WithHTTPPort(port uint16) AppOpts {
	return func(p *appParams) { p.httpAddr = &net.TCPAddr{Port: int(port)} }
}

func WithZepKey(key string) AppOpts {
	return func(p *appParams) { p.zepKey = key }
}

func WithTraceEndpoint(endpoint string) AppOpts {
	return func(p *appParams) { p.traceEndpoint = endpoint }
}

func WithDefaultModelConfig(modelID string) AppOpts {
	return func(p *appParams) { p.defaultModelConfig = modelID }
}

func NewApp(ctx context.Context, opts ...AppOpts) *App {
	p := appParams{}
	for _, opt := range opts {
		opt(&p)
	}
	if err := p.validate(); err != nil {
		panic(err)
	}

	defaultModelConfig := must(ids.NewModelConfigIDFromString(p.defaultModelConfig))

	// push metrics initialization
	m, registerPullMetrics := metrics.RegisterPushMetrics(
		ctx,
		"tghelper",
		"v0.0.0",
		metrics.WithTraceURL(must(url.Parse(p.traceEndpoint))),
	)

	// adapters

	modelConfigs := modelConfigs.New("./models.yaml")
	accountsStorage := file.NewFileTokenStorage("./tokens.yaml")
	serverStorage := serversFile.New(
		"./servers.yaml",
		serversFile.WithTracer(m),
	)
	oauthHandler := oauthClient.New(
		[]string{"mcp.read", "mcp.write"},
		oauthClient.WithTracerProvider(m),
	)

	toolHandler := primitive.NewHandler(oauthHandler, serverStorage, accountsStorage)

	chatStorage := must(zep.NewZepStorage(
		zep.WithAPIKey(p.zepKey),
	))

	gem := must(gemini.NewGeminiModel(
		ctx,
		"gemini-2.5-flash",
		&gemini.ClientConfig{
			APIKey: "<REDACTED>",
		},
	))

	// services

	chatSrv := chat.New(
		chatStorage,
		gem,
		toolHandler,
		serverStorage,
		accountsStorage,
		modelConfigs,
		defaultModelConfig,
	)
	accountSrv := accounts.New(
		serverStorage,
		oauthHandler,
		toolHandler,
		accounts.WithTracerProvider(m),
	)
	serversSrv := servers.New(
		serverStorage,
		oauthHandler,
		must(url.Parse("http://localhost:5002/oauth/callback")),
		servers.WithTracerProvider(m),
	)

	// controllers

	server := newGRPCServer(p.log, m,
		srvWrap(a2a.Register(chatSrv)),
		srvWrap(admin.Register(accountSrv, serversSrv)),
		reflection.Register,
	)

	httpServer := newHTTPServer(
		must(oauth.NewHandler(accountSrv, &url.URL{Scheme: "http", Host: p.httpAddr.String()})),
	)

	// finalizing
	registerPullMetrics(metrics.MetricsPullFuncs{
		// add pull metrics here
	})

	return &App{
		log:        onelog.Wrap(p.log),
		grpcAddr:   p.grpcAddr,
		httpAddr:   p.httpAddr,
		grpcServer: server,
		httpServer: httpServer,
	}
}

func srvWrap(f func(s grpc.ServiceRegistrar)) func(s reflection.GRPCServer) {
	return func(s reflection.GRPCServer) { f(s) }
}
