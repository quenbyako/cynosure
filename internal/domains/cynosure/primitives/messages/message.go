package messages

import (
	"encoding/json"
	"fmt"
	"maps"
)

const (
	maxMessageLength = 8192
)

// Message unifies all message types.
//
// Implemented by these message types:
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

//nolint:exhaustruct // interface check
var (
	_ Message = MessageToolRequest{}
	_ Message = MessageToolResponse{}
	_ Message = MessageToolError{}
	_ Message = MessageAssistant{}
	_ Message = MessageUser{}
)

// MessageTool unifies all message types that are tools.
//
// Implemented by these types:
//
//   - [MessageToolResponse]
//   - [MessageToolError]
type MessageTool interface {
	Message
	ToolCallID() string
	ToolName() string
	Content() json.RawMessage

	_MessageTool()
}

//nolint:exhaustruct // interface check
var (
	_ MessageTool = MessageToolResponse{}
	_ MessageTool = MessageToolError{}
)

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
			var stop bool

			current, stop = handleNextMessage(current, next, err, yield)
			if stop {
				return
			}
		}

		if current != nil {
			yield(current, nil)
		}
	}
}

//nolint:ireturn // intentional interface return
func handleNextMessage(
	current Message,
	next Message,
	err error,
	yield func(Message, error) bool,
) (Message, bool) {
	if err != nil {
		return handleIterError(current, next, err, yield)
	}

	if current == nil {
		return next, false
	}

	if next.MergeTag() != current.MergeTag() {
		if !yield(current, nil) {
			return nil, true
		}

		return next, false
	}

	merged, mergeErr := mergeMessages(current, next)
	if mergeErr != nil {
		if !yield(nil, mergeErr) {
			return nil, true
		}
	}

	return merged, false
}

//nolint:ireturn // intentional interface return
func handleIterError(
	current Message,
	next Message,
	err error,
	yield func(Message, error) bool,
) (Message, bool) {
	if current != nil && !yield(current, err) {
		return nil, true
	}

	yield(next, err)

	return nil, true
}

//nolint:ireturn // intentional interface return
func mergeMessages(current, next Message) (Message, error) {
	switch next := next.(type) {
	case MessageUser:
		return mergeUserMessages(current, next)
	case MessageAssistant:
		return mergeAssistantMessages(current, next)
	default:
		return next, nil
	}
}

//nolint:ireturn // intentional interface return
func mergeUserMessages(current Message, next MessageUser) (Message, error) {
	currentMsg, ok := current.(MessageUser)
	if !ok {
		return nil, ErrInternalValidation("expected previous MessageUser, got %T", current)
	}

	extra := currentMsg.Extra()
	maps.Copy(extra, next.Extra())

	res, err := NewMessageUser(
		currentMsg.Content()+next.Content(),
		WithMessageUserExtra(extra),
		WithMessageUserMergeTag(next.MergeTag()),
	)
	if err != nil {
		return nil, fmt.Errorf("creating merged MessageUser: %w", err)
	}

	return res, nil
}

//nolint:ireturn // intentional interface return
func mergeAssistantMessages(current Message, next MessageAssistant) (Message, error) {
	currentMsg, ok := current.(MessageAssistant)
	if !ok {
		return nil, ErrInternalValidation("expected previous MessageAssistant, got %T", current)
	}

	metadata := next.ProtocolMetadata()
	if metadata == nil {
		metadata = currentMsg.ProtocolMetadata()
	}

	res, err := NewMessageAssistant(
		currentMsg.Content()+next.Content(),
		WithMessageAssistantReasoning(currentMsg.Reasoning()+next.Reasoning()),
		WithMessageAssistantMergeTag(next.MergeTag()),
		WithMessageAssistantAgentID(next.AgentID()),
		WithMessageAssistantProtocolMetadata(metadata),
	)
	if err != nil {
		return nil, fmt.Errorf("creating merged MessageAssistant: %w", err)
	}

	return res, nil
}
