package ids

import (
	"errors"

	"github.com/google/uuid"
)

type UserID struct {
	id uuid.UUID

	valid bool
}

func NewUserIDFromString(id string) (UserID, error) {
	userID, err := uuid.Parse(id)
	if err != nil {
		return UserID{}, err
	}

	return NewUserID(userID)
}

func NewUserID(id uuid.UUID) (UserID, error) {
	u := UserID{
		id: id,
	}

	if err := u.validate(); err != nil {
		return UserID{}, err
	}
	u.valid = true

	return u, nil
}

func (u UserID) Valid() bool { return u.valid || u.validate() == nil }
func (u UserID) validate() error {
	switch {
	case u.id == uuid.Nil:
		return errors.New("invalid user ID")
	default:
		return nil
	}
}

func (u UserID) ID() uuid.UUID { return u.id }
