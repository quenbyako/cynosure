package accounts

import (
	"context"
	"fmt"

	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/adapters/sql/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/accounts"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (a *Accounts) FindAccountsByName(
	ctx context.Context,
	user ids.UserID,
	name string,
) ([]accounts.SearchResult, error) {
	rows, err := a.q.FindAccountsByName(ctx, db.FindAccountsByNameParams{
		UserID: user.ID(),
		Name:   name,
	})
	if err != nil {
		return nil, fmt.Errorf("finding accounts by name: %w", err)
	}

	result := make([]accounts.SearchResult, 0, len(rows))

	for i := range rows {
		res, err := a.parseSearchResult(&rows[i])
		if err != nil {
			return nil, err
		}

		result = append(result, res)
	}

	return result, nil
}

func (a *Accounts) parseSearchResult(
	row *db.FindAccountsByNameRow,
) (accounts.SearchResult, error) {
	accountID, err := a.parseAccountID(row)
	if err != nil {
		return nil, err
	}

	if row.DeletedAt.Valid {
		return accounts.SearchResultDeleted{
			AccountID: accountID,
		}, nil
	}

	acc, err := datatransfer.AccountFromFindAccountsByNameRow(row)
	if err != nil {
		return nil, fmt.Errorf("mapping account: %w", err)
	}

	return accounts.SearchResultActive{
		Account: acc,
	}, nil
}

func (a *Accounts) parseAccountID(
	row *db.FindAccountsByNameRow,
) (ids.AccountID, error) {
	usrID, err := ids.NewUserID(row.UserID)
	if err != nil {
		return ids.AccountID{}, fmt.Errorf("invalid user id: %w", err)
	}

	srvID, err := ids.NewServerID(row.ServerID)
	if err != nil {
		return ids.AccountID{}, fmt.Errorf("invalid server id: %w", err)
	}

	accID, err := ids.NewAccountID(usrID, srvID, row.ID)
	if err != nil {
		return ids.AccountID{}, fmt.Errorf("reconstructing account ID: %w", err)
	}

	return accID, nil
}
