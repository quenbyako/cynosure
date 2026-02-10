package ids

import (
	"errors"

	"github.com/google/uuid"
)

type AccountID struct {
	id     uuid.UUID
	slug   string // optional. empty means it doesn't exists
	user   UserID
	server ServerID

	_valid bool
}

// Implements:
//
// - [WithSlug]
type AccountIDOption interface{ applyAccountID(*AccountID) }

func RandomAccountID(user UserID, server ServerID, opts ...AccountIDOption) (AccountID, error) {
	return NewAccountID(user, server, uuid.New(), opts...)
}

func NewAccountIDFromString(user UserID, server ServerID, id string, opts ...AccountIDOption) (AccountID, error) {
	accountID, err := uuid.Parse(id)
	if err != nil {
		return AccountID{}, err
	}

	return NewAccountID(user, server, accountID, opts...)
}

func NewAccountID(user UserID, server ServerID, id uuid.UUID, opts ...AccountIDOption) (AccountID, error) {
	u := AccountID{
		id:     id,
		user:   user,
		server: server,
	}
	for _, opt := range opts {
		opt.applyAccountID(&u)
	}

	if err := u.validate(); err != nil {
		return AccountID{}, err
	}
	u._valid = true

	return u, nil
}

func (u AccountID) Valid() bool { return u._valid || u.validate() == nil }
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
func (u AccountID) Slug() string     { return u.slug }
func (u AccountID) User() UserID     { return u.user }
func (u AccountID) Server() ServerID { return u.server }
