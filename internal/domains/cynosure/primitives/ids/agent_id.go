package ids

import (
	"fmt"

	"github.com/google/uuid"
)

type AgentID struct {
	id uuid.UUID

	valid bool
}

func RandomModelConfigID() AgentID {
	if id, err := NewModelConfigID(uuid.New()); err == nil {
		return id
	} else {
		panic(err)
	}
}

func NewModelConfigIDFromString(id string) (AgentID, error) {
	modelConfigID, err := uuid.Parse(id)
	if err != nil {
		return AgentID{}, err
	}
	return NewModelConfigID(modelConfigID)
}

func NewModelConfigID(id uuid.UUID) (AgentID, error) {
	t := AgentID{
		id: id,
	}

	if err := t.validate(); err != nil {
		return AgentID{}, err
	}

	t.valid = true

	return t, nil
}

func (u AgentID) Valid() bool { return u.valid || u.validate() == nil }
func (u AgentID) validate() error {
	switch {
	case u.id == uuid.Nil:
		return fmt.Errorf("invalid model config ID: %s", u.id)
	default:
		return nil
	}
}

func (u AgentID) ID() uuid.UUID { return u.id }
