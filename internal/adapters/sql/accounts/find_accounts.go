package accounts

import (
	"context"
	"fmt"

	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/adapters/sql/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (a *Accounts) FindAccountsByName(
	ctx context.Context,
	user ids.UserID,
	name string,
) ([]*entities.Account, error) {
	rows, err := a.q.FindAccountsByName(ctx, db.FindAccountsByNameParams{
		UserID: user.ID(),
		Name:   name,
	})
	if err != nil {
		return nil, fmt.Errorf("finding accounts by name: %w", err)
	}

	result := make([]*entities.Account, 0, len(rows))
	for _, row := range rows {
		acc, err := datatransfer.AccountFromFindAccountsByNameRow(&row)
		if err != nil {
			return nil, fmt.Errorf("mapping account: %w", err)
		}
		result = append(result, acc)
	}

	return result, nil
}
