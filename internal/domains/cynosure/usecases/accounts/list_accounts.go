package accounts

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (s *Usecase) ListAccounts(
	ctx context.Context,
	user ids.UserID,
) ([]entities.AccountReadOnly, error) {
	accountIDs, err := s.accounts.ListAccounts(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("listing accounts: %w", err)
	}

	accounts, err := s.accounts.GetAccountsBatch(ctx, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("getting accounts batch: %w", err)
	}

	result := make([]entities.AccountReadOnly, 0, len(accounts))
	for _, acc := range accounts {
		result = append(result, acc)
	}

	return result, nil
}
