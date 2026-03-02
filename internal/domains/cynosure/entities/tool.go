package entities

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

const embeddingSize = 1536

type ToolCallFunc func(ctx context.Context, params map[string]json.RawMessage) (messages.MessageTool, error)

// Tool is the information of a tool.
type Tool struct {
	id          ids.ToolID
	accountName string
	name        string
	description string

	inputSchema  json.RawMessage
	outputSchema json.RawMessage

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

func NewTool(id ids.ToolID, accountName, name, description string, inputSchema, outputSchema json.RawMessage, opts ...ToolOption) (*Tool, error) {
	normalizedInput, err := normalizeInputSchema(inputSchema)
	if err != nil {
		return nil, err
	}
	normalizedOutput, err := normalizeOutputSchema(outputSchema)
	if err != nil {
		return nil, err
	}

	t := Tool{
		id:           id,
		accountName:  accountName,
		name:         name,
		description:  description,
		inputSchema:  normalizedInput,
		outputSchema: normalizedOutput,
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
	if t.description == "" {
		return errors.New("description is required, but empty")
	}

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
	AccountName() string
	Name() string
	Description() string
	InputSchema() json.RawMessage
	OutputSchema() json.RawMessage
	Embedding() [embeddingSize]float32
}

func (t *Tool) ID() ids.ToolID                    { return t.id }
func (t *Tool) AccountName() string               { return t.accountName }
func (t *Tool) Name() string                      { return t.name }
func (t *Tool) Description() string               { return t.description }
func (t *Tool) InputSchema() json.RawMessage      { return t.inputSchema }
func (t *Tool) OutputSchema() json.RawMessage     { return t.outputSchema }
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
