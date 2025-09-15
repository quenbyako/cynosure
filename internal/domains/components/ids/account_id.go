package ids

import (
	"errors"

	"github.com/google/uuid"
)

type AccountID struct {
	id     uuid.UUID
	user   UserID
	server ServerID

	valid bool
}

func RandomAccountID(user UserID, server ServerID) (AccountID, error) {
	return NewAccountID(user, server, uuid.New())
}

func NewAccountIDFromString(user UserID, server ServerID, id string) (AccountID, error) {
	accountID, err := uuid.Parse(id)
	if err != nil {
		return AccountID{}, err
	}

	return NewAccountID(user, server, accountID)
}

func NewAccountID(user UserID, server ServerID, id uuid.UUID) (AccountID, error) {
	u := AccountID{
		id:     id,
		user:   user,
		server: server,
	}

	if err := u.validate(); err != nil {
		return AccountID{}, err
	}
	u.valid = true

	return u, nil
}

func (u AccountID) Valid() bool { return u.valid || u.validate() == nil }
func (u AccountID) validate() error {
	switch {
	case u.id == uuid.Nil:
		return errors.New("account id cannot be nil")
	case !u.user.Valid():
		return errors.New("user id is invalid")
	case !u.server.Valid():
		return errors.New("server id is invalid")
	default:
		return nil
	}
}

func (u AccountID) ID() uuid.UUID    { return u.id }
func (u AccountID) User() UserID     { return u.user }
func (u AccountID) Server() ServerID { return u.server }
