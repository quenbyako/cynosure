package identitymanager

import (
	"context"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// Port manages user persistence and identity mapping.
type Port interface {
	PortRead
	PortWrite
}

// PortRead defines read operations for user identities.
type PortRead interface {
	// HasUser checks if a user with the given ID exists in the system.
	//
	// See next test suites to find how it works:
	//
	//  - [TestIdentityManager] — verifying existence of saved users.
	//
	// Throws:
	//
	//  - [ErrNotFound]: if the user does not exist.
	HasUser(ctx context.Context, id ids.UserID) (bool, error)

	// LookupUser retrieves a user ID by its external provider and ID.
	//
	// See next test suites to find how it works:
	//
	//  - [TestIdentityManagerLookup] — looking up users by external identity.
	//
	// Throws:
	//
	//  - [ErrNotFound]: if no mapping exists for the given provider and external ID.
	LookupUser(ctx context.Context, telegramID string) (ids.UserID, error)
}

// PortWrite defines write operations for user identities.
type PortWrite interface {
	// CreateUser creates a new user, pushes it to external identity system if
	// necessary, and persists locally. Returns the newly created user ID.
	//
	// TODO: currently only one provider supported — Telegram. It's strictly
	// necessary to add primitives (or entites?) for each user.
	//
	// See next test suites to find how it works:
	//
	//  - [TestCreateUser] — verifying new user creation and data persistence.
	//
	// Throws:
	//
	//  - [ErrAlreadyExists]: if a user with such parameters already exists.
	CreateUser(ctx context.Context, telegramID, nickname, firstName, lastName string) (ids.UserID, error)

	// IssueToken issues a new OAuth2 token for the given user.
	//
	// Throws:
	//
	//  - [ErrNotFound]: if the user does not exist.
	IssueToken(ctx context.Context, id ids.UserID) (*oauth2.Token, error)
}
