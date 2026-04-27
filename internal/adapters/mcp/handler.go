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

const (
	pkgName = "github.com/quenbyako/cynosure/internal/adapters/mcp"
)

type (
	AccountTokenFunc func(context.Context, ids.AccountID) (
		entities.ServerConfigReadOnly, *oauth2.Token, error,
	)
	ServerInfoFunc func(context.Context, ids.ServerID) (entities.ServerConfigReadOnly, error)
	SaveTokenFunc  func(context.Context, ids.AccountID, *oauth2.Token) error
)

const (
	defaultMaxConnSize = 5
	cacheTTL           = 10 * time.Minute
)

var clientImpl = &mcp.Implementation{
	Name:       "cynosure",
	Version:    "0.1.0",
	Title:      "",
	WebsiteURL: "",
	Icons:      nil,
}

type Handler struct {
	clients *cache.Cache[ids.AccountID, *asyncClient]
	tracer  ports.ObserveStack

	// factory and accountToken for probing, bypass cache
	factory *connFactory
}

var _ toolclient.PortFactory = (*Handler)(nil)

// ToolClient implements ports.PortFactory.
func (h *Handler) ToolClient() toolclient.PortWrapped {
	return toolclient.Wrap(h, h.tracer)
}

type handlerParams struct {
	traceProvider core.Metrics
	maxConnSize   uint
}

type HandlerOption func(*handlerParams)

func WithObservability(tp core.Metrics) HandlerOption {
	return func(p *handlerParams) { p.traceProvider = tp }
}

func WithMaxConnSize(size uint) HandlerOption {
	return func(p *handlerParams) { p.maxConnSize = size }
}

//nolint:err113 // new may return unhandlable errors.
func New(
	storage SaveTokenFunc,
	accountToken AccountTokenFunc,
	opts ...HandlerOption,
) (*Handler, error) {
	if storage == nil {
		return nil, errors.New("storage is required")
	}

	if accountToken == nil {
		return nil, errors.New("accountToken is required")
	}

	params := getParams(opts...)

	tracer := ports.StackFromCore(params.traceProvider, pkgName)

	connFactory := NewConnectionFactory(storage, accountToken, tracer.Tracer())

	return &Handler{
		clients: cache.New(
			cacheConstructor(connFactory, accountToken),
			cacheDestructor(),
			params.maxConnSize,
			cacheTTL,
		),
		tracer: tracer,

		factory: connFactory,
	}, nil
}

// Close closes the handler and all active MCP sessions.
func (h *Handler) Close() error {
	if err := h.clients.Close(); err != nil {
		return fmt.Errorf("close clients: %w", err)
	}

	return nil
}

func getParams(opts ...HandlerOption) handlerParams {
	params := handlerParams{
		traceProvider: core.NoopMetrics(),
		maxConnSize:   defaultMaxConnSize,
	}

	for _, opt := range opts {
		opt(&params)
	}

	return params
}

func cacheConstructor(
	factory *connFactory,
	accountToken AccountTokenFunc,
) cache.ConstructorFunc[ids.AccountID, *asyncClient] {
	return func(ctx context.Context, account ids.AccountID) (*asyncClient, error) {
		server, token, err := accountToken(ctx, account)
		if err != nil {
			return nil, fmt.Errorf("retrieve session %v: %w", account.ID().String(), err)
		}

		if token != nil {
			return getAuthorized(ctx, factory, account, server, token)
		}

		if server.AuthConfig() != nil {
			return nil, ErrAuthRequired
		}

		return getAnonymous(ctx, factory, server)
	}
}

func getAuthorized(
	ctx context.Context, factory *connFactory,
	account ids.AccountID, server entities.ServerConfigReadOnly, token *oauth2.Token,
) (*asyncClient, error) {
	client, err := factory.GetAuthorized(ctx, account, server, token)
	if err != nil {
		return nil, fmt.Errorf("get authorized: %w", err)
	}

	return client, nil
}

func getAnonymous(
	ctx context.Context, factory *connFactory, server entities.ServerConfigReadOnly,
) (*asyncClient, error) {
	client, err := factory.GetAnonymous(ctx, server.SSELink(), server.PreferredProtocol())
	if err != nil {
		return nil, fmt.Errorf("get anonymous: %w", err)
	}

	return client, nil
}

func cacheDestructor() cache.DestructorFunc[ids.AccountID, *asyncClient] {
	return func(_ ids.AccountID, client *asyncClient) {
		//nolint:errcheck,gosec // safe to ignore error here
		client.Close()
	}
}
