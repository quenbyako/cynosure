package entities

import (
	"errors"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type UserAccount struct {
	pendingEvents []UserAccountEvent
	userID        ids.UserID
	_valid        bool
}

func NewUser(userID ids.UserID) (*UserAccount, error) {
	userAccount := &UserAccount{
		userID:        userID,
		pendingEvents: nil,
		_valid:        false,
	}

	if err := userAccount.validate(); err != nil {
		return nil, err
	}

	return userAccount, nil
}

func (u *UserAccount) Valid() bool { return u._valid || u.validate() == nil }
func (u *UserAccount) validate() error {
	switch {
	case !u.userID.Valid():
		return errors.New("user ID is invalid")

	default:
		u._valid = true
		return nil
	}
}

type UserAccountEvent interface {
	_UserAccountEvent()
}
