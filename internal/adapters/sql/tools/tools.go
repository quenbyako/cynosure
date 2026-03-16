// Package tools implements SQL tool storage.
package tools

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

const embeddingSize = 1536

type Tools struct {
	tx conn
	q  *db.Queries
}

var _ ports.ToolStorage = (*Tools)(nil)

func New(conn conn) Tools {
	return Tools{
		tx: conn,
		q:  db.New(conn),
	}
}
