package accounts

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/adapters/sql/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/accounts"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (a *Accounts) GetAccount(
	ctx context.Context, account ids.AccountID, opts ...accounts.GetAccountOption,
) (*entities.Account, error) {
	params, err := accounts.GetAccountParams(account, opts...)
	if err != nil {
		return nil, fmt.Errorf("getting account: %w", err)
	}

	row, err := a.getAccount(ctx, params.Account(), params.IncludeDeleted())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ports.ErrNotFound
		}

		return nil, fmt.Errorf("getting account: %w", err)
	}

	acc, err := datatransfer.AccountFromGetAccountRow(&row)
	if err != nil {
		return nil, fmt.Errorf("map account: %w", err)
	}

	return acc, nil
}

func (a *Accounts) getAccount(
	ctx context.Context,
	id ids.AccountID,
	includeDeleted bool,
) (db.GetAccountRow, error) {
	if !includeDeleted {
		row, err := a.q.GetAccount(ctx, id.ID())
		if err != nil {
			return db.GetAccountRow{}, fmt.Errorf("get account: %w", err)
		}

		return row, nil
	}

	res, err := a.q.GetDeletedAccount(ctx, id.ID())
	if err != nil {
		return db.GetAccountRow{}, fmt.Errorf("get account: %w", err)
	}

	return db.GetAccountRow(res), nil
}
