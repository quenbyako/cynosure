package sql

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quenbyako/core"

	"github.com/quenbyako/cynosure/internal/adapters/sql/accounts"
	"github.com/quenbyako/cynosure/internal/adapters/sql/agents"
	"github.com/quenbyako/cynosure/internal/adapters/sql/errors"
	"github.com/quenbyako/cynosure/internal/adapters/sql/quotas"
	"github.com/quenbyako/cynosure/internal/adapters/sql/servers"
	"github.com/quenbyako/cynosure/internal/adapters/sql/threads"
	"github.com/quenbyako/cynosure/internal/adapters/sql/tools"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
)

type Adapter struct {
	accounts.Accounts
	agents.Agents
	servers.Servers
	threads.Threads
	tools.Tools
	observability ports.ObserveStack
	pool          *pgxpool.Pool
	quotas.Quotas
}

var (
	_ ports.AccountStorageFactory = (*Adapter)(nil)
	_ ports.AgentStorageFactory   = (*Adapter)(nil)
	_ ports.ServerStorageFactory  = (*Adapter)(nil)
	_ ports.ThreadStorageFactory  = (*Adapter)(nil)
	_ ports.ToolStorageFactory    = (*Adapter)(nil)
	_ ratelimiter.PortFactory     = (*Adapter)(nil)
	_ io.Closer                   = (*Adapter)(nil)
)

type newParams struct {
	metrics      core.Metrics
	defaultQuota quotas.Config
}

type NewOption func(*newParams)

func WithObservability(m core.Metrics) NewOption {
	return func(p *newParams) { p.metrics = m }
}

func WithDefaultPlanID(planID uuid.UUID) NewOption {
	return func(p *newParams) { p.defaultQuota.DefaultPlanID = planID }
}

func New(ctx context.Context, connString *url.URL, opts ...NewOption) (*Adapter, error) {
	params := newParams{
		metrics:      core.NoopMetrics(),
		defaultQuota: quotas.Config{},
	}

	for _, opt := range opts {
		opt(&params)
	}

	pool, err := initPool(ctx, connString, params.metrics)
	if err != nil {
		return nil, err
	}

	adapter := Adapter{
		Accounts:      accounts.New(pool),
		Agents:        agents.New(pool),
		Servers:       servers.New(pool),
		Threads:       threads.New(pool),
		Tools:         tools.New(pool),
		Quotas:        quotas.New(pool, params.defaultQuota),
		pool:          pool,
		observability: ports.StackFromCore(params.metrics, pkgName),
	}

	if err := adapter.validate(); err != nil {
		return nil, err
	}

	return &adapter, nil
}

func initPool(
	ctx context.Context,
	connString *url.URL,
	observability core.Metrics,
) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connString.String())
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}

	config.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithTracerProvider(observability),
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

	return nil
}

func (a *Adapter) Close() error {
	a.pool.Close()

	return nil
}

func (a *Adapter) WithClock(now func() time.Time) *Adapter {
	a.Quotas = a.Quotas.WithClock(now)
	return a
}

// Factory methods

func (a *Adapter) AccountStorage() ports.AccountStorage { return a }

func (a *Adapter) AgentStorage() ports.AgentStorage { return a }

func (a *Adapter) ServerStorage() ports.ServerStorage { return a }

func (a *Adapter) ThreadStorage() ports.ThreadStorageWrapped {
	return ports.WrapThreadStorage(a, ports.WithTrace(a.observability.Tracer()))
}

func (a *Adapter) ToolStorage() ports.ToolStorage { return a }

func (a *Adapter) RateLimiter() (ratelimiter.PortWrapped, error) {
	return ratelimiter.Wrap(a, a.observability)
}
