package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// Schema defines json schema for input, or output data for different tools.
//
// Core purpose of this component is to inject multi-accounting to model's
// requests. This works through patching of original MCP tool schema.
type Schema struct {
	// TODO: kin-openapi далеко не лучший вариант. Выглядит так, что лучше
	// самостоятельно написать jsonschema тулу
	schema *openapi3.Schema

	// caching validation result for simpler verification on different domain
	// layers
	valid bool
}

// NewSchema creates a new Schema from json raw message.
func NewSchema(raw json.RawMessage) (Schema, error) {
	var parsed openapi3.Schema

	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Schema{}, fmt.Errorf("%w: %w", ErrSchemaParse, err)
	}

	schema := Schema{
		schema: &parsed,
		valid:  false,
	}

	if err := schema.Validate(); err != nil {
		return Schema{}, err
	}

	schema.valid = true

	return schema, nil
}

// Valid reports whether the schema is properly constructed.
func (s Schema) Valid() bool { return s.valid || s.Validate() == nil }

// Validate checks schema invariants.
func (s Schema) Validate() error {
	if s.schema == nil {
		return ErrSchemaNil
	}

	if err := s.schema.Validate(context.Background()); err != nil {
		return fmt.Errorf("%w: %w", ErrSchemaInvalid, err)
	}

	if !s.schema.Type.Is(openapi3.TypeObject) {
		return fmt.Errorf("%w: found %v", ErrSchemaMustBeObject, s.schema.Type)
	}

	return nil
}

// PlainSchema returns the original json schema.
func (s Schema) PlainSchema() json.RawMessage {
	res, err := json.Marshal(s.schema)
	if err != nil {
		//nolint:forbidigo // unreachable, if we checked invariants
		panic(fmt.Sprintf("unreachable: %v", err))
	}

	return res
}
