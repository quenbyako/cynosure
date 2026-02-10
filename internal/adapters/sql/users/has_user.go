package users

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (a *Users) HasUser(ctx context.Context, id ids.UserID) (bool, error) {
	pgID := pgtype.UUID{
		Bytes: id.ID(),
		Valid: true,
	}
	_, err := a.q.GetUser(ctx, pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
