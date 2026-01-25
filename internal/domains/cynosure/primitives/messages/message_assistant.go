package messages

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type MessageAssistant struct {
	mergeTag uint64

	reasoning   string
	text        string
	agentID     ids.AgentID
	attachments []ChatContent

	// Indicates that struct correctly initialized
	valid bool
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

// NewMessageAssistant creates a new assistant message with reasoning, text, and
// optional attachments.
func NewMessageAssistant(text string, opts ...NewMessageAssistantOpt) (MessageAssistant, error) {
	m := MessageAssistant{
		text: text,
	}
	for _, opt := range opts {
		opt(&m)
	}

	if err := m.Validate(); err != nil {
		return MessageAssistant{}, err
	}
	m.valid = true

	return m, nil
}

func (am MessageAssistant) Valid() bool { return am.valid || am.Validate() == nil }
func (am MessageAssistant) Validate() error {
	switch {
	case am.text == "":
		return fmt.Errorf("text cannot be empty")
	case len(am.text) > maxMessageLength:
		return ErrMessageTooLarge
	default:
		return nil
	}
}

func (am MessageAssistant) MergeTag() uint64     { return am.mergeTag }
func (am MessageAssistant) Reasoning() string    { return am.reasoning }
func (am MessageAssistant) Text() string         { return am.text }
func (am MessageAssistant) AgentID() ids.AgentID { return am.agentID }
func (am MessageAssistant) Format(ctx context.Context, vs map[string]any, formatType FormatType) (Message, error) {
	changedText, err := formatContent(am.text, vs, formatType)
	if err != nil {
		return nil, fmt.Errorf("format assistant message text: %w", err)
	}

	return MessageAssistant{
		reasoning:   am.reasoning,
		text:        changedText,
		attachments: am.attachments,
	}, nil
}
