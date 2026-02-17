package sql

import (
	"context"
	"fmt"
	"io"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/trace"
	noopTrace "go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/adapters/sql/accounts"
	"github.com/quenbyako/cynosure/internal/adapters/sql/agents"
	"github.com/quenbyako/cynosure/internal/adapters/sql/servers"
	"github.com/quenbyako/cynosure/internal/adapters/sql/threads"
	"github.com/quenbyako/cynosure/internal/adapters/sql/tools"
	"github.com/quenbyako/cynosure/internal/adapters/sql/users"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

const pkgName = "github.com/quenbyako/cynosure/internal/adapters/sql"

type Adapter struct {
	accounts.Accounts
	agents.Agents
	servers.Servers
	threads.Threads
	tools.Tools
	users.Users

	pool *pgxpool.Pool

	trace trace.Tracer
}

var _ ports.AccountStorageFactory = (*Adapter)(nil)
var _ ports.AgentStorageFactory = (*Adapter)(nil)
var _ ports.ServerStorageFactory = (*Adapter)(nil)
var _ ports.ThreadStorageFactory = (*Adapter)(nil)
var _ ports.ToolStorageFactory = (*Adapter)(nil)
var _ ports.UserStorageFactory = (*Adapter)(nil)
var _ io.Closer = (*Adapter)(nil)

type newParams struct {
	tracer trace.TracerProvider
}

type NewOption func(*newParams)

func WithTrace(tp trace.TracerProvider) NewOption {
	return func(p *newParams) {
		if tp == nil {
			panic("tracer provider is nil")
		}
		p.tracer = tp
	}
}

func New(ctx context.Context, connString string, opts ...NewOption) (*Adapter, error) {
	p := newParams{
		tracer: noopTrace.NewTracerProvider(),
	}
	for _, opt := range opts {
		opt(&p)
	}

	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}
	config.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithTracerProvider(p.tracer),
	)

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("creating pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging db: %w", err)
	}

	a := Adapter{
		Accounts: accounts.New(pool),
		Agents:   agents.New(pool),
		Servers:  servers.New(pool),
		Threads:  threads.New(pool),
		Tools:    tools.New(pool),
		Users:    users.New(pool),
		pool:     pool,
		trace:    p.tracer.Tracer(pkgName),
	}
	if err := a.validate(); err != nil {
		return nil, err
	}

	return &a, nil
}

func (a *Adapter) validate() error {
	if a.pool == nil {
		return fmt.Errorf("pool is nil")
	}
	if a.trace == nil {
		return fmt.Errorf("trace is nil")
	}
	return nil
}

func (a *Adapter) Close() error {
	a.pool.Close()
	return nil
}

// Factory methods

func (a *Adapter) AccountStorage() ports.AccountStorage { return a }
func (a *Adapter) AgentStorage() ports.AgentStorage     { return a }
func (a *Adapter) ServerStorage() ports.ServerStorage   { return a }
func (a *Adapter) ThreadStorage() ports.ThreadStorageWrapped {
	return ports.WrapThreadStorage(a, ports.WithTrace(a.trace))
}
func (a *Adapter) ToolStorage() ports.ToolStorage { return a }
func (a *Adapter) UserStorage() ports.UserStorage { return a }
