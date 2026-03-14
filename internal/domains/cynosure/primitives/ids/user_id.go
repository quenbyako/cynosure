package ids

import (
	"errors"

	"github.com/google/uuid"
)

type UserID struct {
	id uuid.UUID

	_valid bool
}

func RandomUserID() UserID {
	if id, err := NewUserID(uuid.New()); err == nil {
		return id
	} else {
		panic(err)
	}
}

func NewUserIDFromString(id string) (UserID, error) {
	userID, err := uuid.Parse(id)
	if err != nil {
		return UserID{}, err
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
	switch u.id {
	case uuid.Nil:
		return errors.New("invalid user ID")
	default:
		return nil
	}
}

func (u UserID) ID() uuid.UUID { return u.id }
