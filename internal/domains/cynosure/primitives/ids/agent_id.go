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
	if id, err := NewAgentID(user, uuid.New()); err != nil {
		return AgentID{}, err
	} else {
		return id, nil
	}
}

func NewAgentIDFromString(user UserID, id string) (AgentID, error) {
	modelConfigID, err := uuid.Parse(id)
	if err != nil {
		return AgentID{}, err
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
	if u.id == uuid.Nil {
		return fmt.Errorf("invalid model config ID: %s", u.id)
	}

	if !u.user.Valid() {
		return fmt.Errorf("invalid user ID: %v", u.user.ID().String())
	}

	return nil
}

func (u AgentID) ID() uuid.UUID  { return u.id }
func (u AgentID) UserID() UserID { return u.user }
