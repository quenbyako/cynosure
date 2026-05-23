package accounts

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (a *Accounts) ReactivateAccount(ctx context.Context, account ids.AccountID) error {
	transaction, err := a.tx.BeginTx(ctx, emptyTxOptions)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	//nolint:errcheck // Rollback is called in defer and it makes no sense to check the error
	defer transaction.Rollback(ctx)

	queries := a.q.WithTx(transaction)

	if err := queries.ReactivateAccount(ctx, account.ID()); err != nil {
		return fmt.Errorf("reactivating account: %w", err)
	}

	if err := queries.ReactivateAccountTools(ctx, account.ID()); err != nil {
		return fmt.Errorf("reactivating account tools: %w", err)
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}
