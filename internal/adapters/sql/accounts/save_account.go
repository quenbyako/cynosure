package accounts

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
)

func (a *Accounts) SaveAccount(ctx context.Context, info entities.AccountReadOnly) error {
	tx, err := a.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := a.q.WithTx(tx)

	err = q.UpsertAccount(ctx, db.UpsertAccountParams{
		ID:          info.ID().ID(),
		UserID:      info.ID().User().ID(),
		ServerID:    info.ID().Server().ID(),
		Name:        info.Name(),
		Description: info.Description(),
	})
	if err != nil {
		return fmt.Errorf("upserting account: %w", err)
	}

	// Handle OAuth token - always delete old and upsert new (one-to-one relation)
	if err := q.DeleteAccountToken(ctx, info.ID().ID()); err != nil {
		return fmt.Errorf("deleting old oauth token: %w", err)
	}

	if token := info.Token(); token != nil {
		var tokenType *string
		if token.TokenType != "" {
			tokenType = ptr(token.TokenType)
		}

		var refreshToken *string

		if token.RefreshToken != "" {
			refreshToken = ptr(token.RefreshToken)
		}

		if err := q.AddOAuthToken(ctx, db.AddOAuthTokenParams{
			AccountID:    info.ID().ID(),
			Type:         tokenType,
			AccessToken:  token.AccessToken,
			RefreshToken: refreshToken,
			Expiry: pgtype.Timestamptz{
				Time:  token.Expiry,
				Valid: !token.Expiry.IsZero(),
			},
		}); err != nil {
			return fmt.Errorf("adding oauth token: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}
