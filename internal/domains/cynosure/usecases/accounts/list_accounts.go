package accounts

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (s *Usecase) ListAccounts(ctx context.Context, user ids.UserID) ([]ids.AccountID, error) {
	t, err := s.accounts.ListAccounts(ctx, user)
	if err != nil {
		return nil, err
	}

	return t, nil
}
