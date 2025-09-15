package tools

import (
	"encoding/json"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// ToolInfo is the information of a tool.
type ToolInfo struct {
	name string
	desc string

	params   json.RawMessage
	response json.RawMessage

	valid bool // indicates if the tool info is valid
}

func NewToolInfo(name, desc string, paramsSchema, responseSchema json.RawMessage) (ToolInfo, error) {
	normalizedInput, err := normalizeInputSchema(paramsSchema)
	if err != nil {
		return ToolInfo{}, err
	}
	normalizedOutput, err := normalizeOutputSchema(responseSchema)
	if err != nil {
		return ToolInfo{}, err
	}

	t := ToolInfo{
		name:     name,
		desc:     desc,
		params:   normalizedInput,
		response: normalizedOutput,
	}

	if err := t.Validate(); err != nil {
		return ToolInfo{}, err
	}
	t.valid = true

	return t, nil
}

func (t ToolInfo) Valid() bool { return t.valid || t.Validate() == nil }
func (t ToolInfo) Validate() error {
	switch {
	default:
		return nil
	}
}

func normalizeInputSchema(schema json.RawMessage) (json.RawMessage, error) {
	var parsedSchema openapi3.Schema
	if err := json.Unmarshal(schema, &parsedSchema); err != nil {
		return nil, fmt.Errorf("cannot parse schema: %w", err)
	}
	if !parsedSchema.Type.Is(openapi3.TypeObject) {
		return nil, fmt.Errorf("invalid schema type: %s, must be only object", parsedSchema.Type)
	}
	schema, err := json.Marshal(&parsedSchema)
	if err != nil {
		panic(fmt.Errorf("cannot marshal schema back: %w", err))
	}

	return schema, nil
}

func normalizeOutputSchema(schema json.RawMessage) (json.RawMessage, error) {
	var parsedSchema openapi3.Schema
	if err := json.Unmarshal(schema, &parsedSchema); err != nil {
		return nil, fmt.Errorf("cannot parse schema: %w", err)
	}
	schema, err := json.Marshal(&parsedSchema)
	if err != nil {
		panic(fmt.Errorf("cannot marshal schema back: %w", err))
	}

	return schema, nil
}

func (t ToolInfo) Name() string                    { return t.name }
func (t ToolInfo) Desc() string                    { return t.desc }
func (t ToolInfo) ParamsSchema() json.RawMessage   { return t.params }
func (t ToolInfo) ResponseSchema() json.RawMessage { return t.response }
