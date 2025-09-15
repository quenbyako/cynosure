package ids

import (
	"fmt"

	"github.com/google/uuid"
)

type ServerID struct {
	id uuid.UUID

	valid bool
}

func RandomServerID() ServerID {
	if id, err := NewServerID(uuid.New()); err == nil {
		return id
	} else {
		panic(err)
	}
}

func NewServerIDFromString(id string) (ServerID, error) {
	serverID, err := uuid.Parse(id)
	if err != nil {
		return ServerID{}, err
	}
	return NewServerID(serverID)
}

func NewServerID(id uuid.UUID) (ServerID, error) {
	t := ServerID{
		id: id,
	}

	if err := t.validate(); err != nil {
		return ServerID{}, err
	}

	t.valid = true

	return t, nil
}

func (u ServerID) Valid() bool { return u.valid || u.validate() == nil }
func (u ServerID) validate() error {
	switch {
	case u.id == uuid.Nil:
		return fmt.Errorf("invalid server ID: %s", u.id)
	default:
		return nil
	}
}

func (u ServerID) ID() uuid.UUID { return u.id }
