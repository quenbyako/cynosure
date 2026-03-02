package accounts

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (s *Usecase) ListAccounts(ctx context.Context, user ids.UserID) ([]entities.AccountReadOnly, error) {
	ids, err := s.accounts.ListAccounts(ctx, user)
	if err != nil {
		return nil, err
	}

	accounts, err := s.accounts.GetAccountsBatch(ctx, ids)
	if err != nil {
		return nil, err
	}

	result := make([]entities.AccountReadOnly, 0, len(accounts))
	for _, acc := range accounts {
		result = append(result, acc)
	}

	return result, nil
}
