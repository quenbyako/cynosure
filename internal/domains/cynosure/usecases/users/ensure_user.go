package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// EnsureUser ensures that a user exists in the system by checking their
// external identity. If the user doesn't exist, it creates a new user, pushes
// it to Ory, and initializes their default environment.
func (u *Usecase) EnsureUser(
	ctx context.Context,
	externalID, nickname, firstName, lastName string,
) (ids.UserID, error) {
	userID, err := u.getOrCreateUser(ctx, externalID, nickname, firstName, lastName)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("getting user: %w", err)
	}

	// 3. Initialize account (Admin MCP and Meta-Agent)
	// This is now idempotent and will only perform missing steps.
	if err := u.InitializeAccount(ctx, userID); err != nil {
		// Log error but maybe don't fail completely if we have at least USER created?
		// Actually, without agents/tools the bot won't work well.
		return userID, fmt.Errorf("initializing user account: %w", err)
	}

	return userID, nil
}

func (u *Usecase) getOrCreateUser(ctx context.Context,
	externalID, nickname, firstName, lastName string,
) (ids.UserID, error) {
	// 1. Try to lookup user by identity
	userID, err := u.users.LookupUser(ctx, externalID)
	if err == nil {
		return userID, nil
	}

	if !errors.Is(err, identitymanager.ErrNotFound) {
		return ids.UserID{}, fmt.Errorf("looking up user: %w", err)
	}

	// 2. If not found, create a new user
	userID, err = u.users.CreateUser(ctx, externalID, nickname, firstName, lastName)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("creating user: %w", err)
	}

	return userID, nil
}
