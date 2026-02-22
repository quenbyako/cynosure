package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// EnsureUser ensures that a user exists in the system by checking their external identity.
// If the user doesn't exist, it creates a new user, pushes it to Ory, and initializes their default environment.
func (s *Usecase) EnsureUser(ctx context.Context, externalID, nickname, firstName, lastName string) (ids.UserID, error) {
	// 1. Try to lookup user by identity
	userID, err := s.users.LookupUser(ctx, externalID)
	if err == nil {
		return userID, nil
	} else if !errors.Is(err, ports.ErrNotFound) {
		return ids.UserID{}, fmt.Errorf("looking up user: %w", err)
	}

	// 2. If not found, create a new user
	newUserID, err := s.users.CreateUser(ctx, externalID, nickname, firstName, lastName)
	if err != nil {
		return ids.UserID{}, err
	}

	// 3. Initialize default environment (Meta-Agent)
	// TODO: Implement meta-agent creation after defining default agent settings
	return newUserID, nil
}
