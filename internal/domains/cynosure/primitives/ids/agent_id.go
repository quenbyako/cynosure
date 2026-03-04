package ids

import (
	"fmt"

	"github.com/google/uuid"
)

type AgentID struct {
	user UserID
	id   uuid.UUID

	valid bool
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
	t := AgentID{
		user: user,
		id:   id,
	}

	if err := t.validate(); err != nil {
		return AgentID{}, err
	}

	t.valid = true

	return t, nil
}

func (u AgentID) Valid() bool { return u.valid || u.validate() == nil }
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
