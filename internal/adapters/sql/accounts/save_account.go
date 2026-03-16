package accounts

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
)

var emptyTxOptions pgx.TxOptions

func (a *Accounts) SaveAccount(ctx context.Context, info entities.AccountReadOnly) error {
	transaction, err := a.tx.BeginTx(ctx, emptyTxOptions)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	//nolint:errcheck // Rollback is called in defer and it makes no sense to check the error
	defer transaction.Rollback(ctx)

	queries := a.q.WithTx(transaction)

	if err := queries.UpsertAccount(ctx, db.UpsertAccountParams{
		ID:          info.ID().ID(),
		UserID:      info.ID().User().ID(),
		ServerID:    info.ID().Server().ID(),
		Name:        info.Name(),
		Description: info.Description(),
		Embedding:   nil,
	}); err != nil {
		return fmt.Errorf("upserting account: %w", err)
	}

	if err := a.upsertToken(ctx, queries, info); err != nil {
		return err
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func (a *Accounts) upsertToken(
	ctx context.Context,
	queries *db.Queries,
	info entities.AccountReadOnly,
) error {
	if err := queries.DeleteAccountToken(ctx, info.ID().ID()); err != nil {
		return fmt.Errorf("deleting old oauth token: %w", err)
	}

	token := info.Token()
	if token == nil {
		return nil
	}

	params := db.AddOAuthTokenParams{
		AccountID:    info.ID().ID(),
		Type:         optional(token.TokenType),
		AccessToken:  token.AccessToken,
		RefreshToken: optional(token.RefreshToken),
		Expiry: pgtype.Timestamptz{
			Time:             token.Expiry,
			Valid:            !token.Expiry.IsZero(),
			InfinityModifier: pgtype.Finite,
		},
	}

	if err := queries.AddOAuthToken(ctx, params); err != nil {
		return fmt.Errorf("adding oauth token: %w", err)
	}

	return nil
}

func optional[T comparable](value T) *T {
	var zero T
	if value == zero {
		return nil
	}

	return &value
}
