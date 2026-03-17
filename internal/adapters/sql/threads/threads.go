// Package threads implements SQL thread storage.
package threads

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

type Threads struct {
	tx conn
	q  *db.Queries
}

var _ ports.ThreadStorage = (*Threads)(nil)

func New(conn conn) Threads {
	return Threads{
		tx: conn,
		q:  db.New(conn),
	}
}
