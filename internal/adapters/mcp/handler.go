package mcp

import (
	"context"
	"fmt"

	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	cache "github.com/quenbyako/cynosure/contrib/sf-cache"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const pkgName = "github.com/quenbyako/cynosure/internal/adapters/mcp"

type (
	AccountTokenFunc func(context.Context, ids.AccountID) (entities.ServerConfigReadOnly, *oauth2.Token, error)
	ServerInfoFunc   func(context.Context, ids.ServerID) (entities.ServerConfigReadOnly, error)
	SaveTokenFunc    func(context.Context, ids.AccountID, *oauth2.Token) error
)

var clientImpl = &mcp.Implementation{
	Name:    "cynosure",
	Version: "0.1.0",
}

type Handler struct {
	clients *cache.Cache[ids.AccountID, *asyncClient]
	tracer  trace.Tracer

	// factory and accountToken for probing, bypass cache
	factory      *connFactory
	accountToken AccountTokenFunc
}

var _ ports.ToolClientFactory = (*Handler)(nil)

func (h *Handler) ToolClient() ports.ToolClientWrapped {
	return ports.WrapToolClient(h, ports.WithToolClientTrace(h.tracer))
}

type handlerParams struct {
	tp          trace.TracerProvider
	maxConnSize uint
}

type HandlerOption func(*handlerParams)

func WithTracerProvider(tp trace.TracerProvider) HandlerOption {
	return func(p *handlerParams) { p.tp = tp }
}

func WithMaxConnSize(size uint) HandlerOption {
	return func(p *handlerParams) { p.maxConnSize = size }
}

func New(
	storage SaveTokenFunc,
	accountToken AccountTokenFunc,
	opts ...HandlerOption,
) *Handler {
	if storage == nil {
		panic("storage is required")
	}
	if accountToken == nil {
		panic("accountToken is required")
	}

	p := handlerParams{
		tp:          noop.NewTracerProvider(),
		maxConnSize: 5,
	}
	for _, opt := range opts {
		opt(&p)
	}

	tracer := p.tp.Tracer(pkgName)

	connFactory := NewConnectionFactory(storage, accountToken, tracer)

	return &Handler{
		clients: cache.New(
			cacheConstructor(connFactory, accountToken),
			cacheDestructor(),
			p.maxConnSize,
			10*time.Minute,
		),
		tracer: tracer,

		factory: connFactory,
	}
}

func cacheConstructor(
	factory *connFactory,
	accountToken AccountTokenFunc,
) cache.ConstructorFunc[ids.AccountID, *asyncClient] {
	return func(ctx context.Context, account ids.AccountID) (*asyncClient, error) {
		server, token, err := accountToken(ctx, account)
		if err != nil {
			return nil, fmt.Errorf("retrieving session for account %v: %w", account.ID().String(), err)
		}

		return factory.GetAuthorized(ctx, account, server, token)
	}
}

func cacheDestructor() cache.DestructorFunc[ids.AccountID, *asyncClient] {
	return func(_ ids.AccountID, c *asyncClient) {
		if err := c.Close(); err != nil {
			panic(fmt.Errorf("closing client: %w (i'll handle error later)", err))
		}
	}
}
