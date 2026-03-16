package sql

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/trace"
	noopTrace "go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/adapters/sql/accounts"
	"github.com/quenbyako/cynosure/internal/adapters/sql/agents"
	"github.com/quenbyako/cynosure/internal/adapters/sql/errors"
	"github.com/quenbyako/cynosure/internal/adapters/sql/servers"
	"github.com/quenbyako/cynosure/internal/adapters/sql/threads"
	"github.com/quenbyako/cynosure/internal/adapters/sql/tools"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

type Adapter struct {
	accounts.Accounts
	agents.Agents
	servers.Servers
	threads.Threads
	tools.Tools

	pool *pgxpool.Pool

	trace trace.Tracer
}

var (
	_ ports.AccountStorageFactory = (*Adapter)(nil)
	_ ports.AgentStorageFactory   = (*Adapter)(nil)
	_ ports.ServerStorageFactory  = (*Adapter)(nil)
	_ ports.ThreadStorageFactory  = (*Adapter)(nil)
	_ ports.ToolStorageFactory    = (*Adapter)(nil)
	_ io.Closer                   = (*Adapter)(nil)
)

type newParams struct {
	tracer trace.TracerProvider
}

type NewOption func(*newParams)

func WithTrace(tracerProvider trace.TracerProvider) NewOption {
	return func(p *newParams) { p.tracer = tracerProvider }
}

func New(ctx context.Context, connString *url.URL, opts ...NewOption) (*Adapter, error) {
	params := newParams{
		tracer: noopTrace.NewTracerProvider(),
	}

	for _, opt := range opts {
		opt(&params)
	}

	pool, err := initPool(ctx, connString, params.tracer)
	if err != nil {
		return nil, err
	}

	adapter := Adapter{
		Accounts: accounts.New(pool),
		Agents:   agents.New(pool),
		Servers:  servers.New(pool),
		Threads:  threads.New(pool),
		Tools:    tools.New(pool),
		pool:     pool,
		trace:    params.tracer.Tracer(pkgName),
	}

	if err := adapter.validate(); err != nil {
		return nil, err
	}

	return &adapter, nil
}

func initPool(
	ctx context.Context,
	connString *url.URL,
	tracerProvider trace.TracerProvider,
) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connString.String())
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}

	config.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithTracerProvider(tracerProvider),
	)

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("creating pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("pinging db: %w", err)
	}

	return pool, nil
}

func (a *Adapter) validate() error {
	if a.pool == nil {
		return errors.ErrPoolNil
	}

	if a.trace == nil {
		return errors.ErrTraceNil
	}

	return nil
}

func (a *Adapter) Close() error {
	a.pool.Close()

	return nil
}

// Factory methods

//nolint:ireturn // returning interface is intended for factory methods
func (a *Adapter) AccountStorage() ports.AccountStorage { return a }

//nolint:ireturn // returning interface is intended for factory methods
func (a *Adapter) AgentStorage() ports.AgentStorage { return a }

//nolint:ireturn // returning interface is intended for factory methods
func (a *Adapter) ServerStorage() ports.ServerStorage { return a }

//nolint:ireturn // returning interface is intended for factory methods
func (a *Adapter) ThreadStorage() ports.ThreadStorageWrapped {
	return ports.WrapThreadStorage(a, ports.WithTrace(a.trace))
}

//nolint:ireturn // returning interface is intended for factory methods
func (a *Adapter) ToolStorage() ports.ToolStorage { return a }
