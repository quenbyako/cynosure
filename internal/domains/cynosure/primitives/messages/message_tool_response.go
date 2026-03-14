package messages

import (
	"encoding/json"
	"errors"
)

type MessageToolResponse struct {
	toolName   string
	toolCallID string
	content    json.RawMessage
	mergeTag   uint64
	_valid     bool // Indicates that struct correctly initialized
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

	m._valid = true

	return m, nil
}

func (tm MessageToolResponse) Valid() bool { return tm._valid || tm.Validate() == nil }
func (tm MessageToolResponse) Validate() error {
	switch {
	case tm.toolName == "":
		return errors.New("tool name cannot be empty")
	case tm.toolCallID == "":
		return errors.New("tool call ID cannot be empty")
	case !json.Valid(tm.content):
		return errors.New("content must be valid JSON")
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
