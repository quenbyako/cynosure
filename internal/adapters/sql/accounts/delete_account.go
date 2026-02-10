package accounts

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (a *Accounts) DeleteAccount(ctx context.Context, account ids.AccountID) error {
	if err := a.q.SoftDeleteAccount(ctx, account.ID()); err != nil {
		return fmt.Errorf("soft deleting account: %w", err)
	}

	return nil
}
