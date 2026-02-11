package users

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (a *Users) NewUserWithIdentity(ctx context.Context, id ids.UserID, provider string, externalID string) error {
	if provider != "telegram" {
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	telegramID, err := strconv.ParseInt(externalID, 10, 64)
	if err != nil {
		return fmt.Errorf("parsing telegram id: %w", err)
	}

	tx, err := a.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := a.q.WithTx(tx)

	err = q.CreateUser(ctx, id.ID())
	if err != nil {
		return fmt.Errorf("creating user: %w", err)
	}

	err = q.CreateUserTelegram(ctx, db.CreateUserTelegramParams{
		UserID:     id.ID(),
		TelegramID: telegramID,
	})
	if err != nil {
		return fmt.Errorf("creating user telegram: %w", err)
	}

	return tx.Commit(ctx)
}
