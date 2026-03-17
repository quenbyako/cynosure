// Package agents implements SQL agent storage.
package agents

import (
	"context"

	"github.com/jackc/pgx/v5"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

type conn interface {
	db.DBTX
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type Agents struct {
	tx conn
	q  *db.Queries
}

var _ ports.AgentStorage = (*Agents)(nil)

func New(conn conn) Agents {
	return Agents{
		tx: conn,
		q:  db.New(conn),
	}
}
