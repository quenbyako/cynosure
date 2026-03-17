// Package ids defines domain identifiers.
package ids //nolint:dupl // typed ID pattern: structurally identical to ServerID by design

import (
	"fmt"

	"github.com/google/uuid"
)

type UserID struct {
	id uuid.UUID

	_valid bool
}

// RandomUserID returns a new random UserID.
// uuid.New() always produces a non-nil UUID so this never fails.
func RandomUserID() UserID {
	return UserID{id: uuid.New(), _valid: true}
}

func NewUserIDFromString(id string) (UserID, error) {
	userID, err := uuid.Parse(id)
	if err != nil {
		return UserID{}, fmt.Errorf("parsing user id: %w", err)
	}

	return NewUserID(userID)
}

func NewUserID(id uuid.UUID) (UserID, error) {
	user := UserID{
		id:     id,
		_valid: false,
	}

	if err := user.validate(); err != nil {
		return UserID{}, err
	}

	user._valid = true

	return user, nil
}

func (u UserID) Valid() bool { return u._valid || u.validate() == nil }
func (u UserID) validate() error {
	if u.id == uuid.Nil {
		return ErrInternalValidation("user id cannot be nil")
	}

	return nil
}

func (u UserID) ID() uuid.UUID { return u.id }
