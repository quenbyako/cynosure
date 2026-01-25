package entities

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

const embeddingSize = 1536

type ToolCallFunc func(ctx context.Context, params map[string]json.RawMessage) (messages.MessageTool, error)

// Tool is the information of a tool.
type Tool struct {
	id   ids.ToolID
	name string
	desc string

	params   json.RawMessage
	response json.RawMessage

	embedding [embeddingSize]float32

	// meta fields

	pendingEvents[ToolEvent]
	_valid bool // indicates if the tool info is valid
}

var _ EventsReader[ToolEvent] = (*Tool)(nil)
var _ ToolReadOnly = (*Tool)(nil)

type ToolOption func(*Tool)

func WithEmbedding(embedding [embeddingSize]float32) ToolOption {
	return func(t *Tool) { t.embedding = embedding }
}

func NewTool(id ids.ToolID, name, desc string, paramsSchema, responseSchema json.RawMessage, opts ...ToolOption) (*Tool, error) {
	normalizedInput, err := normalizeInputSchema(paramsSchema)
	if err != nil {
		return nil, err
	}
	normalizedOutput, err := normalizeOutputSchema(responseSchema)
	if err != nil {
		return nil, err
	}

	t := Tool{
		id:       id,
		name:     name,
		desc:     desc,
		params:   normalizedInput,
		response: normalizedOutput,
	}
	for _, opt := range opts {
		opt(&t)
	}

	if err := t.Validate(); err != nil {
		return nil, err
	}
	t._valid = true

	return &t, nil
}

// VALIDATION

func (t *Tool) Valid() bool { return t._valid || t.Validate() == nil }
func (t *Tool) Validate() error {
	return nil
}

// NORMALIZATION

func normalizeInputSchema(schema json.RawMessage) (json.RawMessage, error) {
	return normalizeSchema(schema, openapi3.TypeObject)
}

func normalizeOutputSchema(schema json.RawMessage) (json.RawMessage, error) {
	return normalizeSchema(schema, "")
}

func normalizeSchema(schema json.RawMessage, verifyRoot string) (json.RawMessage, error) {
	var parsedSchema openapi3.Schema
	if err := json.Unmarshal(schema, &parsedSchema); err != nil {
		return nil, fmt.Errorf("cannot parse schema: %w", err)
	}
	if verifyRoot != "" && !parsedSchema.Type.Is(verifyRoot) {
		return nil, fmt.Errorf("invalid schema type: %s, must be only object", parsedSchema.Type)
	}
	schema, err := json.Marshal(&parsedSchema)
	if err != nil {
		panic(fmt.Errorf("cannot marshal schema back: %w", err))
	}

	return schema, nil
}

// READ

type ToolReadOnly interface {
	ID() ids.ToolID
	Name() string
	Desc() string
	ParamsSchema() json.RawMessage
	ResponseSchema() json.RawMessage
	Embedding() [embeddingSize]float32
}

func (t *Tool) ID() ids.ToolID                    { return t.id }
func (t *Tool) Name() string                      { return t.name }
func (t *Tool) Desc() string                      { return t.desc }
func (t *Tool) ParamsSchema() json.RawMessage     { return t.params }
func (t *Tool) ResponseSchema() json.RawMessage   { return t.response }
func (t *Tool) Embedding() [embeddingSize]float32 { return t.embedding }

// WRITE

func (t *Tool) SetEmbedding(embedding [embeddingSize]float32) {
	previous := t.embedding
	t.embedding = embedding

	t.pendingEvents = append(t.pendingEvents, ToolEventEmbeddingUpdated{
		previous:  previous,
		embedding: embedding,
	})
}

// EVENTS

// [ToolEventEmbeddingUpdated]
type ToolEvent interface {
	_ToolEvent()
}

type ToolEventEmbeddingUpdated struct {
	previous  [embeddingSize]float32
	embedding [embeddingSize]float32
}

func (e ToolEventEmbeddingUpdated) _ToolEvent() {}

func (e ToolEventEmbeddingUpdated) Embedding() [embeddingSize]float32 { return e.embedding }
