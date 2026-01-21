package sql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/quenbyako/cynosure/contrib/db/gen/go"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

type Adapter struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

var _ ports.AccountStorageFactory = (*Adapter)(nil)
var _ ports.ModelSettingsStorageFactory = (*Adapter)(nil)
var _ ports.ServerStorageFactory = (*Adapter)(nil)

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
		pool: pool,
		q:    db.New(pool),
	}, nil
}

func (a *Adapter) AccountStorage() ports.AccountStorage             { return a }
func (a *Adapter) ModelSettingsStorage() ports.ModelSettingsStorage { return a }
func (a *Adapter) ServerStorage() ports.ServerStorage               { return a }

func (a *Adapter) Close() error {
	a.pool.Close()

	return nil
}
