package ids

import (
	"fmt"

	"github.com/google/uuid"
)

type ToolID struct {
	id      uuid.UUID
	account AccountID

	_valid bool
}

func RandomToolID(account AccountID) (ToolID, error) {
	return NewToolID(account, uuid.New())
}

func NewToolIDFromString(account AccountID, id string) (ToolID, error) {
	toolID, err := uuid.Parse(id)
	if err != nil {
		return ToolID{}, fmt.Errorf("parsing tool id: %w", err)
	}

	return NewToolID(account, toolID)
}

func NewToolID(account AccountID, id uuid.UUID) (ToolID, error) {
	tool := ToolID{
		id:      id,
		account: account,
		_valid:  false,
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
		return ErrInternalValidation("tool id cannot be nil")
	case !u.account.Valid():
		return ErrInternalValidation("account id is invalid")
	default:
		return nil
	}
}

func (u ToolID) ID() uuid.UUID      { return u.id }
func (u ToolID) Account() AccountID { return u.account }
