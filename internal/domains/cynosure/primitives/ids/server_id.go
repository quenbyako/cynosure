package ids //nolint:dupl // TODO: need to come up with something, how to reduce duplication

import (
	"fmt"

	"github.com/google/uuid"
)

type ServerID struct {
	id uuid.UUID

	_valid bool
}

// RandomServerID returns a new random ServerID.
// uuid.New() always produces a non-nil UUID so this never fails.
func RandomServerID() ServerID {
	return ServerID{id: uuid.New(), _valid: true}
}

func NewServerIDFromString(id string) (ServerID, error) {
	serverID, err := uuid.Parse(id)
	if err != nil {
		return ServerID{}, fmt.Errorf("parsing server id: %w", err)
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
	if u.id == uuid.Nil {
		return ErrInternalValidation("server id cannot be nil")
	}

	return nil
}

func (u ServerID) ID() uuid.UUID { return u.id }
