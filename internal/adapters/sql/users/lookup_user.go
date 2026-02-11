package users

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (a *Users) LookupUser(ctx context.Context, provider string, externalID string) (ids.UserID, error) {
	if provider != "telegram" {
		return ids.UserID{}, fmt.Errorf("unsupported provider: %s", provider)
	}

	telegramID, err := strconv.ParseInt(externalID, 10, 64)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("parsing telegram id: %w", err)
	}

	userID, err := a.q.LookupUserByTelegram(ctx, telegramID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ids.UserID{}, ports.ErrNotFound
		}
		return ids.UserID{}, fmt.Errorf("looking up user by telegram: %w", err)
	}

	return ids.NewUserID(userID)
}
