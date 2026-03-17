package ids

import (
	"fmt"

	"github.com/google/uuid"
)

type AgentID struct {
	user UserID
	id   uuid.UUID

	_valid bool
}

func RandomAgentID(user UserID) (AgentID, error) {
	return NewAgentID(user, uuid.New())
}

func NewAgentIDFromString(user UserID, id string) (AgentID, error) {
	modelConfigID, err := uuid.Parse(id)
	if err != nil {
		return AgentID{}, fmt.Errorf("parsing agent id: %w", err)
	}

	return NewAgentID(user, modelConfigID)
}

func NewAgentID(user UserID, id uuid.UUID) (AgentID, error) {
	agent := AgentID{
		user:   user,
		id:     id,
		_valid: false,
	}

	if err := agent.validate(); err != nil {
		return AgentID{}, err
	}

	agent._valid = true

	return agent, nil
}

func (u AgentID) Valid() bool { return u._valid || u.validate() == nil }
func (u AgentID) validate() error {
	switch {
	case u.id == uuid.Nil:
		return ErrInternalValidation("invalid model config ID: %s", u.id)
	case !u.user.Valid():
		return ErrInternalValidation("invalid user ID: %v", u.user.ID().String())
	default:
		return nil
	}
}

func (u AgentID) ID() uuid.UUID  { return u.id }
func (u AgentID) UserID() UserID { return u.user }
