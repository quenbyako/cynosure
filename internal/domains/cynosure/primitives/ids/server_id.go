package ids

import (
	"fmt"

	"github.com/google/uuid"
)

type ServerID struct {
	id uuid.UUID

	_valid bool
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
	server := ServerID{
		id:     id,
		_valid: false,
	}

	if err := server.validate(); err != nil {
		return ServerID{}, err
	}

	server._valid = true

	return server, nil
}

func (u ServerID) Valid() bool { return u._valid || u.validate() == nil }
func (u ServerID) validate() error {
	switch u.id {
	case uuid.Nil:
		return fmt.Errorf("invalid server ID: %s", u.id)
	default:
		return nil
	}
}

func (u ServerID) ID() uuid.UUID { return u.id }
