// Package accounts implements SQL account storage.
package accounts

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

type Accounts struct {
	tx conn
	q  *db.Queries
}

var _ ports.AccountStorage = (*Accounts)(nil)

func New(conn conn) Accounts {
	return Accounts{
		tx: conn,
		q:  db.New(conn),
	}
}
