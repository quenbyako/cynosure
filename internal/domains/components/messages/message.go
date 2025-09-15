package messages

import (
	"encoding/json"
	"fmt"
	"iter"
	"maps"
)

const maxMessageLength = 2048

// Implemented by all message types:
//
//   - [MessageToolRequest]
//   - [MessageToolResponse]
//   - [MessageToolError]
//   - [MessageAssistant]
//   - [MessageUser]
type Message interface {
	MergeTag() uint64
	Valid() bool
	Validate() error

	_Message()
}

var _ Message = MessageToolRequest{}
var _ Message = MessageToolResponse{}
var _ Message = MessageToolError{}
var _ Message = MessageAssistant{}
var _ Message = MessageUser{}

type MessageTool interface {
	Message

	_MessageTool()
}

var _ MessageTool = MessageToolResponse{}
var _ MessageTool = MessageToolError{}

func validateExtra(extra map[string]json.RawMessage) bool {
	for _, v := range extra {
		if !json.Valid(v) {
			return false
		}
	}

	return true
}

func MergeMessagesStreaming(messages iter.Seq2[Message, error]) iter.Seq2[Message, error] {
	return func(yield func(Message, error) bool) {
		var current Message
		for next, err := range messages {
			if err != nil {
				if current != nil && !yield(current, err) {
					return // stop iteration on error
				}

				yield(next, err)
				return
			}

			if current == nil {
				current = next
				continue
			}

			if next.MergeTag() != current.MergeTag() {
				if !yield(current, nil) {
					return
				}

				current = next
				continue
			}

			switch next := next.(type) {
			case MessageUser:
				currentMsg, ok := current.(MessageUser)
				if !ok {
					yield(nil, fmt.Errorf("expected previous MessageUser, got %T", currentMsg))
					return
				}

				extra := currentMsg.Extra()
				maps.Copy(extra, next.Extra())

				if current, err = NewMessageUser(
					currentMsg.Content()+next.Content(),
					WithMessageUserExtra(extra),
					WithMessageUserMergeTag(next.MergeTag()),
				); err != nil {
					yield(nil, fmt.Errorf("creating merged MessageUser: %w", err))
				}

			case MessageAssistant:
				currentMsg, ok := current.(MessageAssistant)
				if !ok {
					yield(nil, fmt.Errorf("expected previous MessageAssistant, got %T", next))
					return
				}

				if current, err = NewMessageAssistant(
					currentMsg.Text()+next.Text(),
					WithMessageAssistantReasoning(currentMsg.Reasoning()+next.Reasoning()),
					WithMessageAssistantMergeTag(next.MergeTag()),
				); err != nil {
					yield(nil, fmt.Errorf("creating merged MessageAssistant: %w", err))
				}

			default:
				if !yield(current, nil) {
					return // stop iteration if yield returns false
				}

				current = next
				continue
			}
		}

		if current != nil {
			yield(current, nil)
		}
	}
}
