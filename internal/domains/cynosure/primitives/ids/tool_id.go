package ids

import (
	"errors"

	"github.com/google/uuid"
)

type ToolID struct {
	id      uuid.UUID
	account AccountID

	_valid bool
}

// Implelements:
// - [WithSlug]
type ToolIDOption interface {
	applyToolID(*ToolID)
}

func RandomToolID(account AccountID, opts ...ToolIDOption) (ToolID, error) {
	return NewToolID(account, uuid.New(), opts...)
}

func NewToolIDFromString(account AccountID, id string, opts ...ToolIDOption) (ToolID, error) {
	toolID, err := uuid.Parse(id)
	if err != nil {
		return ToolID{}, err
	}

	return NewToolID(account, toolID, opts...)
}

func NewToolID(account AccountID, id uuid.UUID, opts ...ToolIDOption) (ToolID, error) {
	u := ToolID{
		id:      id,
		account: account,
	}
	for _, opt := range opts {
		opt.applyToolID(&u)
	}

	if err := u.validate(); err != nil {
		return ToolID{}, err
	}
	u._valid = true

	return u, nil
}

func (u ToolID) Valid() bool { return u._valid || u.validate() == nil }
func (u ToolID) validate() error {
	switch {
	case u.id == uuid.Nil:
		return errors.New("tool id cannot be nil")
	case !u.account.Valid():
		return errors.New("account id is invalid")
	default:
		return nil
	}
}

func (u ToolID) ID() uuid.UUID      { return u.id }
func (u ToolID) Account() AccountID { return u.account }
