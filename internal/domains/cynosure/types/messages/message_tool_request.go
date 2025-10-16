package messages

import (
	"encoding/json"
	"fmt"
)

type MessageToolRequest struct {
	mergeTag uint64

	reasoning string
	// TODO: очень неочевидное поведение: вообще-то, просто тула не может
	// существовать без аккаунта. Но как это запихнуть в структуру — не очень
	// понятно. По идее, надо добавить сюда именно [AccountID], а так же
	// РАЗВЕРНУТЫЕ в обратку аргументы (аргументы могут содержать в себе данные
	// об аккаунте). Пока непонятно как с этим жить, придется как нибудь.
	toolName   string
	toolCallID string
	arguments  map[string]json.RawMessage

	// Indicates that struct correctly initialized
	valid bool
}

func (tm MessageToolRequest) _Message() {}

type NewMessageToolRequestOpt func(*MessageToolRequest)

func WithMessageToolRequestMergeTag(mergeTag uint64) NewMessageToolRequestOpt {
	return func(m *MessageToolRequest) { m.mergeTag = mergeTag }
}

func WithMessageToolRequestReasoning(reasoning string) NewMessageToolRequestOpt {
	return func(m *MessageToolRequest) { m.reasoning = reasoning }
}

func NewMessageToolRequest(arguments map[string]json.RawMessage, toolName, toolCallID string, opts ...NewMessageToolRequestOpt) (MessageToolRequest, error) {
	m := MessageToolRequest{
		toolName:   toolName,
		toolCallID: toolCallID,
		arguments:  arguments,
	}
	for _, opt := range opts {
		opt(&m)
	}

	if err := m.Validate(); err != nil {
		return MessageToolRequest{}, err
	}
	m.valid = true

	return m, nil
}

func (tm MessageToolRequest) Valid() bool { return tm.valid || tm.Validate() == nil }
func (tm MessageToolRequest) Validate() error {
	encodedArgs, err := json.Marshal(tm.arguments)
	if err != nil {
		return fmt.Errorf("arguments must be valid JSON: %w", err)
	}

	switch {
	case tm.toolName == "":
		return fmt.Errorf("tool name cannot be empty")

	case tm.toolCallID == "":
		return fmt.Errorf("tool call ID cannot be empty")

	case len(encodedArgs) > maxMessageLength:
		return ErrMessageTooLarge

	default:
		return nil
	}
}

func (tm MessageToolRequest) Reasoning() string                     { return tm.reasoning }
func (tm MessageToolRequest) MergeTag() uint64                      { return tm.mergeTag }
func (tm MessageToolRequest) ToolName() string                      { return tm.toolName }
func (tm MessageToolRequest) ToolCallID() string                    { return tm.toolCallID }
func (tm MessageToolRequest) Arguments() map[string]json.RawMessage { return tm.arguments }
