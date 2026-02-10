package messages

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
)

type MessageUser struct {
	mergeTag uint64

	content string

	extra map[string]json.RawMessage

	// Indicates that struct correctly initialized
	valid bool
}

func (m MessageUser) _Message() {}

type NewMessageUserOpt func(*MessageUser)

func WithMessageUserExtra(extra map[string]json.RawMessage) NewMessageUserOpt {
	return func(m *MessageUser) { m.extra = extra }
}

func WithMessageUserMergeTag(mergeTag uint64) NewMessageUserOpt {
	return func(m *MessageUser) { m.mergeTag = mergeTag }
}

func NewMessageUser(content string, opts ...NewMessageUserOpt) (MessageUser, error) {
	m := MessageUser{
		content: content,
	}
	for _, opt := range opts {
		opt(&m)
	}

	if err := m.Validate(); err != nil {
		return MessageUser{}, err
	}
	m.valid = true

	return m, nil
}

func (m MessageUser) Valid() bool { return m.valid || m.Validate() == nil }
func (m MessageUser) Validate() error {
	switch {
	case m.content == "":
		return fmt.Errorf("content cannot be empty")
	case len(m.content) > maxMessageLength:
		return ErrMessageTooLarge
	case !validateExtra(m.extra):
		return fmt.Errorf("extra must be valid JSON")
	default:
		return nil
	}
}

func (m MessageUser) MergeTag() uint64                  { return m.mergeTag }
func (m MessageUser) Content() string                   { return m.content }
func (m MessageUser) Extra() map[string]json.RawMessage { return m.extra }
func (m MessageUser) Format(ctx context.Context, vs map[string]any, formatType FormatType) (Message, error) {
	changed, err := formatContent(m.content, vs, formatType)
	if err != nil {
		return nil, fmt.Errorf("format user message content: %w", err)
	}

	return &MessageUser{
		content: changed,
		extra:   maps.Clone(m.extra),
	}, nil
}
