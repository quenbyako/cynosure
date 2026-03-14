package messages

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type MessageAssistant struct {
	reasoning   string
	content     string
	attachments []ChatContent
	// TODO: выпилить нахуй отсюда, это очень временное решение, просто чтоб
	// попробовать. сюда запихивается thought sig от gemini, и не сохраняется в
	// базу (и ни в коем случае не должен!)
	protocolMetadata []byte
	mergeTag         uint64
	agentID          ids.AgentID
	_valid           bool // Indicates that struct correctly initialized
}

func (am MessageAssistant) _Message() {}

type NewMessageAssistantOpt func(*MessageAssistant)

func WithMessageAssistantMergeTag(mergeTag uint64) NewMessageAssistantOpt {
	return func(m *MessageAssistant) { m.mergeTag = mergeTag }
}

func WithMessageAssistantAttachments(attachments ...ChatContent) NewMessageAssistantOpt {
	return func(m *MessageAssistant) { m.attachments = append(m.attachments, attachments...) }
}

func WithMessageAssistantReasoning(reasoning string) NewMessageAssistantOpt {
	return func(m *MessageAssistant) { m.reasoning = reasoning }
}

func WithMessageAssistantAgentID(agentID ids.AgentID) NewMessageAssistantOpt {
	return func(m *MessageAssistant) { m.agentID = agentID }
}

func WithMessageAssistantProtocolMetadata(metadata []byte) NewMessageAssistantOpt {
	return func(m *MessageAssistant) { m.protocolMetadata = metadata }
}

// NewMessageAssistant creates a new assistant message with reasoning, text, and
// optional attachments.
func NewMessageAssistant(content string, opts ...NewMessageAssistantOpt) (MessageAssistant, error) {
	m := MessageAssistant{
		content: content,
	}
	for _, opt := range opts {
		opt(&m)
	}

	if err := m.Validate(); err != nil {
		return MessageAssistant{}, err
	}

	m._valid = true

	return m, nil
}

func (am MessageAssistant) Valid() bool { return am._valid || am.Validate() == nil }
func (am MessageAssistant) Validate() error {
	switch {
	case am.content == "":
		return errors.New("text cannot be empty")
	case len(am.content) > maxMessageLength:
		return ErrMessageTooLarge
	default:
		return nil
	}
}

func (am MessageAssistant) MergeTag() uint64         { return am.mergeTag }
func (am MessageAssistant) Reasoning() string        { return am.reasoning }
func (am MessageAssistant) Content() string          { return am.content }
func (am MessageAssistant) AgentID() ids.AgentID     { return am.agentID }
func (am MessageAssistant) ProtocolMetadata() []byte { return bytes.Clone(am.protocolMetadata) }
func (am MessageAssistant) Format(ctx context.Context, vs map[string]any, formatType FormatType) (Message, error) {
	changedText, err := formatContent(am.content, vs, formatType)
	if err != nil {
		return nil, fmt.Errorf("format assistant message text: %w", err)
	}

	// TODO: NewMessageAssistant, not just copy-paste. it might be invalid!
	return MessageAssistant{
		mergeTag:         am.mergeTag,
		reasoning:        am.reasoning,
		content:          changedText,
		agentID:          am.agentID,
		attachments:      am.attachments,
		protocolMetadata: am.protocolMetadata,
		_valid:           true,
	}, nil
}
