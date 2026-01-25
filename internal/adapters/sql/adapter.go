package sql

import (
	"context"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quenbyako/cynosure/internal/adapters/sql/accounts"
	"github.com/quenbyako/cynosure/internal/adapters/sql/agents"
	"github.com/quenbyako/cynosure/internal/adapters/sql/servers"
	"github.com/quenbyako/cynosure/internal/adapters/sql/threads"
	"github.com/quenbyako/cynosure/internal/adapters/sql/tools"
	"github.com/quenbyako/cynosure/internal/adapters/sql/users"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

type Adapter struct {
	accounts.Accounts
	agents.Agents
	servers.Servers
	threads.Threads
	tools.Tools
	users.Users

	pool *pgxpool.Pool
}

var _ ports.AccountStorage = (*Adapter)(nil)
var _ ports.AgentStorage = (*Adapter)(nil)
var _ ports.ServerStorage = (*Adapter)(nil)
var _ ports.ThreadStorage = (*Adapter)(nil)
var _ ports.ToolStorage = (*Adapter)(nil)
var _ ports.UserStorage = (*Adapter)(nil)
var _ io.Closer = (*Adapter)(nil)

func NewAdapter(ctx context.Context, connString string) (*Adapter, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("creating pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging db: %w", err)
	}

	return &Adapter{
		Accounts: accounts.New(pool),
		Agents:   agents.New(pool),
		Servers:  servers.New(pool),
		Threads:  threads.New(pool),
		Tools:    tools.New(pool),
		Users:    users.New(pool),
		pool:     pool,
	}, nil
}

func (a *Adapter) Close() error {
	a.pool.Close()
	return nil
}

// Factory methods

func (a *Adapter) AccountStorage() ports.AccountStorage { return a }
func (a *Adapter) AgentStorage() ports.AgentStorage     { return a }
func (a *Adapter) ServerStorage() ports.ServerStorage   { return a }
func (a *Adapter) ThreadStorage() ports.ThreadStorage   { return a }
func (a *Adapter) ToolStorage() ports.ToolStorage       { return a }
func (a *Adapter) UserStorage() ports.UserStorage       { return a }
