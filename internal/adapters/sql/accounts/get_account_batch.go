package accounts

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/quenbyako/cynosure/internal/adapters/sql/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (a *Accounts) GetAccountsBatch(ctx context.Context, accounts []ids.AccountID) ([]*entities.Account, error) {
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
	for i, row := range rows {
		acc, err := datatransfer.AccountFromGetAccountsBatchRow(row)
		if err != nil {
			return nil, fmt.Errorf("mapping account %s: %w", row.ID, err)
		}
		result[i] = acc
	}

	return result, nil
}
