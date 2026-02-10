package messages

import (
	"encoding/json"
	"fmt"
)

type MessageToolResponse struct {
	mergeTag uint64

	toolName   string
	toolCallID string
	content    json.RawMessage

	// Indicates that struct correctly initialized
	valid bool
}

func (tm MessageToolResponse) _Message()     {}
func (tm MessageToolResponse) _MessageTool() {}

type NewMessageToolResponseOpt func(*MessageToolResponse)

func WithMessageToolResponseMergeTag(mergeTag uint64) NewMessageToolResponseOpt {
	return func(m *MessageToolResponse) { m.mergeTag = mergeTag }
}

func NewMessageToolResponse(content json.RawMessage, toolName, toolCallID string, opts ...NewMessageToolResponseOpt) (MessageToolResponse, error) {
	m := MessageToolResponse{
		toolName:   toolName,
		toolCallID: toolCallID,
		content:    json.RawMessage(content),
	}

	for _, opt := range opts {
		opt(&m)
	}

	if err := m.Validate(); err != nil {
		return MessageToolResponse{}, err
	}
	m.valid = true

	return m, nil
}

func (tm MessageToolResponse) Valid() bool { return tm.valid || tm.Validate() == nil }
func (tm MessageToolResponse) Validate() error {
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

func (tm MessageToolResponse) MergeTag() uint64         { return tm.mergeTag }
func (tm MessageToolResponse) ToolName() string         { return tm.toolName }
func (tm MessageToolResponse) ToolCallID() string       { return tm.toolCallID }
func (tm MessageToolResponse) Content() json.RawMessage { return tm.content }
