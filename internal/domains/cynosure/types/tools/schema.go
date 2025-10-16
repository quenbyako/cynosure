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

func NewSchema(schema json.RawMessage) (s Schema, err error) {
	var parsedSchema openapi3.Schema
	if err := json.Unmarshal(schema, &parsedSchema); err != nil {
		return Schema{}, fmt.Errorf("cannot parse schema: %w", err)
	}
	if !parsedSchema.Type.Is(openapi3.TypeObject) {
		return Schema{}, fmt.Errorf("invalid schema type: %s, must be only object", parsedSchema.Type)
	}
	if schema, err = json.Marshal(&parsedSchema); err != nil {
		panic(fmt.Errorf("cannot marshal schema back: %w", err))
	}

	s = Schema{
		schema: &parsedSchema,
	}

	if err := s.Validate(); err != nil {
		return Schema{}, err
	}
	s.valid = true

	return s, nil
}

func (s Schema) Valid() bool { return s.valid || s.Validate() == nil }
func (s Schema) Validate() error {
	if s.schema == nil {
		return fmt.Errorf("schema must not be nil")
	} else if err := s.schema.Validate(context.Background()); err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	} else if !s.schema.Type.Is(openapi3.TypeObject) {
		return fmt.Errorf("invalid schema type: %s, must be only object", s.schema.Type)
	}

	return nil
}

func (s Schema) PlainSchema() json.RawMessage { return must(json.Marshal(s.schema)) }

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
