package messages

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type MessageToolRequest struct {
	arguments map[string]json.RawMessage
	reasoning string
	// TODO: очень неочевидное поведение: вообще-то, просто тула не может
	// существовать без аккаунта. Но как это запихнуть в структуру — не очень
	// понятно. По идее, надо добавить сюда именно [AccountID], а так же
	// РАЗВЕРНУТЫЕ в обратку аргументы (аргументы могут содержать в себе данные
	// об аккаунте). Пока непонятно как с этим жить, придется как нибудь.
	toolName   string
	toolCallID string
	// TODO: выпилить нахуй отсюда, это очень временное решение, просто чтоб
	// попробовать. сюда запихивается thought sig от gemini, и не сохраняется в
	// базу (и ни в коем случае не должен!)
	protocolMetadata []byte
	mergeTag         uint64
	_valid           bool // Indicates that struct correctly initialized
}

func (tm MessageToolRequest) _Message() {}

type NewMessageToolRequestOpt func(*MessageToolRequest)

func WithMessageToolRequestMergeTag(mergeTag uint64) NewMessageToolRequestOpt {
	return func(m *MessageToolRequest) { m.mergeTag = mergeTag }
}

func WithMessageToolRequestReasoning(reasoning string) NewMessageToolRequestOpt {
	return func(m *MessageToolRequest) { m.reasoning = reasoning }
}

func WithMessageToolRequestProtocolMetadata(metadata []byte) NewMessageToolRequestOpt {
	return func(m *MessageToolRequest) { m.protocolMetadata = metadata }
}

func NewMessageToolRequest(
	arguments map[string]json.RawMessage,
	toolName, toolCallID string,
	opts ...NewMessageToolRequestOpt,
) (MessageToolRequest, error) {
	message := MessageToolRequest{
		toolName:         toolName,
		toolCallID:       toolCallID,
		arguments:        arguments,
		reasoning:        "",
		protocolMetadata: nil,
		mergeTag:         0,
		_valid:           false,
	}
	for _, opt := range opts {
		opt(&message)
	}

	if err := message.Validate(); err != nil {
		return MessageToolRequest{}, err
	}

	message._valid = true

	return message, nil
}

func (tm MessageToolRequest) Valid() bool { return tm._valid || tm.Validate() == nil }
func (tm MessageToolRequest) Validate() error {
	encodedArgs, err := json.Marshal(tm.arguments)
	if err != nil {
		return fmt.Errorf("arguments must be valid JSON: %w", err)
	}

	switch {
	case tm.toolName == "":
		return ErrInternalValidation("tool name cannot be empty")

	case tm.toolCallID == "":
		return ErrInternalValidation("tool call ID cannot be empty")

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
func (tm MessageToolRequest) ProtocolMetadata() []byte {
	return bytes.Clone(tm.protocolMetadata)
}
