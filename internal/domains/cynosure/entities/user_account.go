package entities

import (
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
)

type UserAccount struct {
	userID ids.UserID

	pendingEvents []UserAccountEvent
	valid         bool
}

func NewUser(userID ids.UserID) (*UserAccount, error) {
	u := &UserAccount{
		userID: userID,
	}

	if err := u.validate(); err != nil {
		return nil, err
	}

	return u, nil
}

func (u *UserAccount) Valid() bool { return u.valid || u.validate() == nil }
func (u *UserAccount) validate() error {
	switch {
	case !u.userID.Valid():
		return fmt.Errorf("user ID is invalid")

	default:
		u.valid = true
		return nil
	}
}

type UserAccountEvent interface {
	_UserAccountEvent()
}
