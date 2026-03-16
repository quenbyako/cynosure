package accounts

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/quenbyako/cynosure/internal/adapters/sql/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (a *Accounts) GetAccountsBatch(
	ctx context.Context, accounts []ids.AccountID,
) ([]*entities.Account, error) {
	if len(accounts) == 0 {
		return []*entities.Account{}, nil
	}

	uuids := make([]uuid.UUID, len(accounts))
	for i, acc := range accounts {
		uuids[i] = acc.ID()
	}

	rows, err := a.q.GetAccountsBatch(ctx, uuids)
	if err != nil {
		return nil, fmt.Errorf("batch getting accounts: %w", err)
	}

	result := make([]*entities.Account, len(rows))
	for i := range rows { // not using value to omit copying
		acc, err := datatransfer.AccountFromGetAccountsBatchRow(&rows[i])
		if err != nil {
			return nil, fmt.Errorf("mapping account %s: %w", rows[i].ID, err)
		}

		result[i] = acc
	}

	return result, nil
}
