package messages

import (
	"context"
	"encoding/json"
	"fmt"
)

type MessageToolError struct {
	mergeTag uint64

	toolName   string
	toolCallID string
	content    json.RawMessage

	// Indicates that struct correctly initialized
	valid bool
}

func (tm MessageToolError) _Message()     {}
func (tm MessageToolError) _MessageTool() {}

type NewMessageToolErrorOpt func(*MessageToolError)

func WithMessageToolErrorMergeTag(mergeTag uint64) NewMessageToolErrorOpt {
	return func(m *MessageToolError) { m.mergeTag = mergeTag }
}

func NewMessageToolError(content json.RawMessage, toolName, toolCallID string, opts ...NewMessageToolErrorOpt) (MessageToolError, error) {
	m := MessageToolError{
		toolName:   toolName,
		toolCallID: toolCallID,
		content:    json.RawMessage(content),
	}

	for _, opt := range opts {
		opt(&m)
	}

	if err := m.Validate(); err != nil {
		return MessageToolError{}, err
	}
	m.valid = true

	return m, nil
}

func (tm MessageToolError) Valid() bool { return tm.valid || tm.Validate() == nil }
func (tm MessageToolError) Validate() error {
	switch {
	case tm.toolName == "":
		return fmt.Errorf("tool name cannot be empty")

	case tm.toolCallID == "":
		return fmt.Errorf("tool call ID cannot be empty")

	case !json.Valid(tm.content):
		return fmt.Errorf("content must be valid JSON")

	case len(tm.content) > maxMessageLength:
		return ErrMessageTooLarge

	default:
		return nil
	}
}

func (tm MessageToolError) MergeTag() uint64         { return tm.mergeTag }
func (tm MessageToolError) ToolName() string         { return tm.toolName }
func (tm MessageToolError) ToolCallID() string       { return tm.toolCallID }
func (tm MessageToolError) Content() json.RawMessage { return tm.content }
func (tm MessageToolError) Format(ctx context.Context, vs map[string]any, formatType FormatType) (Message, error) {
	return nil, fmt.Errorf("tool message cannot be formatted")
}
