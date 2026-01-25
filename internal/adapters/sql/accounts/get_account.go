package accounts

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/quenbyako/cynosure/internal/adapters/sql/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (a *Accounts) GetAccount(ctx context.Context, account ids.AccountID) (*entities.Account, error) {
	row, err := a.q.GetAccount(ctx, account.ID())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ports.ErrNotFound
		}
		return nil, fmt.Errorf("getting account: %w", err)
	}

	acc, err := datatransfer.AccountFromGetAccountRow(row)
	if err != nil {
		return nil, fmt.Errorf("map account: %w", err)
	}
	return acc, nil
}
