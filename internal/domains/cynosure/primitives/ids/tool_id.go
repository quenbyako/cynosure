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
	tool := ToolID{
		id:      id,
		account: account,
		_valid:  false,
	}
	for _, opt := range opts {
		opt.applyToolID(&tool)
	}

	if err := tool.validate(); err != nil {
		return ToolID{}, err
	}

	tool._valid = true

	return tool, nil
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
