package messages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
)

type MessageUser struct {
	extra    map[string]json.RawMessage
	content  string
	mergeTag uint64
	_valid   bool // Indicates that struct correctly initialized
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

	m._valid = true

	return m, nil
}

func (m MessageUser) Valid() bool { return m._valid || m.Validate() == nil }
func (m MessageUser) Validate() error {
	switch {
	case m.content == "":
		return errors.New("content cannot be empty")
	case len(m.content) > maxMessageLength:
		return ErrMessageTooLarge
	case !validateExtra(m.extra):
		return errors.New("extra must be valid JSON")
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
		return MessageUser{}, fmt.Errorf("format user message content: %w", err)
	}

	return MessageUser{
		mergeTag: m.mergeTag,
		content:  changed,
		extra:    maps.Clone(m.extra),
		_valid:   true,
	}, nil
}
