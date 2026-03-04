package accounts

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (a *Accounts) ListAccounts(ctx context.Context, user ids.UserID) ([]ids.AccountID, error) {
	rows, err := a.q.ListAccountIDs(ctx, user.ID())
	if err != nil {
		return nil, fmt.Errorf("listing accounts: %w", err)
	}

	result := make([]ids.AccountID, 0, len(rows))
	for _, row := range rows {
		serverID, err := ids.NewServerID(row.ServerID)
		if err != nil {
			return nil, fmt.Errorf("invalid server id for %s: %w", row.ID, err)
		}

		id, err := ids.NewAccountID(user, serverID, row.ID)
		if err != nil {
			return nil, fmt.Errorf("constructing account id for %s: %w", row.ID, err)
		}

		result = append(result, id)
	}

	return result, nil
}
