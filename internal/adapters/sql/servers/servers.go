// Package servers implements SQL server storage.
package servers

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

type Servers struct {
	tx conn
	q  *db.Queries
}

var _ ports.ServerStorage = (*Servers)(nil)

func New(conn conn) Servers {
	return Servers{
		tx: conn,
		q:  db.New(conn),
	}
}
