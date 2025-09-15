package ids

import (
	"fmt"

	"github.com/google/uuid"
)

type ModelConfigID struct {
	id uuid.UUID

	valid bool
}

func RandomModelConfigID() ModelConfigID {
	if id, err := NewModelConfigID(uuid.New()); err == nil {
		return id
	} else {
		panic(err)
	}
}

func NewModelConfigIDFromString(id string) (ModelConfigID, error) {
	modelConfigID, err := uuid.Parse(id)
	if err != nil {
		return ModelConfigID{}, err
	}
	return NewModelConfigID(modelConfigID)
}

func NewModelConfigID(id uuid.UUID) (ModelConfigID, error) {
	t := ModelConfigID{
		id: id,
	}

	if err := t.validate(); err != nil {
		return ModelConfigID{}, err
	}

	t.valid = true

	return t, nil
}

func (u ModelConfigID) Valid() bool { return u.valid || u.validate() == nil }
func (u ModelConfigID) validate() error {
	switch {
	case u.id == uuid.Nil:
		return fmt.Errorf("invalid model config ID: %s", u.id)
	default:
		return nil
	}
}

func (u ModelConfigID) ID() uuid.UUID { return u.id }
