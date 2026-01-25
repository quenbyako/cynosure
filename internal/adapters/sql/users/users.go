package users

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

type Users struct {
	tx conn
	q  *db.Queries
}

var _ ports.UserStorage = (*Users)(nil)

func New(conn conn) Users {
	return Users{
		tx: conn,
		q:  db.New(conn),
	}
}
