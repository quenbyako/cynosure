package mcp

import (
	"context"
	"errors"
	"fmt"

	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/quenbyako/core"
	cache "github.com/quenbyako/cynosure/contrib/sf-cache"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
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
	tracer  ports.ObserveStack

	// factory and accountToken for probing, bypass cache
	factory      *connFactory
	accountToken AccountTokenFunc
}

var _ toolclient.PortFactory = (*Handler)(nil)

func (h *Handler) ToolClient() toolclient.PortWrapped {
	return toolclient.Wrap(h, h.tracer)
}

type handlerParams struct {
	tp          core.Metrics
	maxConnSize uint
}

type HandlerOption func(*handlerParams)

func WithObservability(tp core.Metrics) HandlerOption {
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
		tp:          core.NoopMetrics(),
		maxConnSize: 5,
	}
	for _, opt := range opts {
		opt(&p)
	}

	tracer := ports.StackFromCore(p.tp, pkgName)

	connFactory := NewConnectionFactory(storage, accountToken, tracer.Tracer())

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

		if token != nil {
			return factory.GetAuthorized(ctx, account, server, token)
		}

		if server.AuthConfig() != nil {
			return nil, errors.New("server requires auth, however, it's not provided")
		}

		return factory.GetAnonymous(ctx, server.SSELink(), server.PreferredProtocol())
	}
}

func cacheDestructor() cache.DestructorFunc[ids.AccountID, *asyncClient] {
	return func(_ ids.AccountID, c *asyncClient) {
		if err := c.Close(); err != nil {
			panic(fmt.Errorf("closing client: %w (i'll handle error later)", err))
		}
	}
}
