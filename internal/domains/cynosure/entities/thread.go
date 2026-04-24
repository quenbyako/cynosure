package entities

import (
	"fmt"
	"slices"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

type Thread struct {
	messages []messages.Message
	pendingEvents[ThreadEvent]
	id      ids.ThreadID
	agentID ids.AgentID
	_valid  bool
}

var (
	_ EventsReader[ThreadEvent] = (*Thread)(nil)
	_ ThreadReadOnly            = (*Thread)(nil)
)

type ThreadOption func(*Thread)

func WithAgent(id ids.AgentID) ThreadOption {
	return func(t *Thread) { t.agentID = id }
}

func NewThread(
	id ids.ThreadID,
	history []messages.Message,
	opts ...ThreadOption,
) (*Thread, error) {
	thread := &Thread{
		id:            id,
		messages:      history,
		pendingEvents: nil,
		agentID:       ids.AgentID{},
		_valid:        false,
	}
	for _, opt := range opts {
		opt(thread)
	}

	if err := thread.Validate(); err != nil {
		return nil, err
	}

	thread._valid = true

	return thread, nil
}

func (c *Thread) Valid() bool { return c._valid || c.Validate() == nil }
func (c *Thread) Validate() error {
	if !c.id.Valid() {
		return ErrInternalValidation("thread ID is invalid")
	}

	if len(c.messages) == 0 {
		return ErrInternalValidation("messages cannot be empty")
	}

	return nil
}

func (c *Thread) validateMessages(history []messages.Message) error {
	if len(history) == 0 {
		return ErrInternalValidation("messages cannot be empty")
	}

	for i, msg := range history {
		if !msg.Valid() {
			return ErrInternalValidation("message %d is invalid", i)
		}
	}

	return nil
}

// CHANGES

func (c *Thread) Synchronized() bool           { return len(c.pendingEvents) == 0 }
func (c *Thread) PendingEvents() []ThreadEvent { return slices.Clone(c.pendingEvents) }
func (c *Thread) ClearEvents()                 { c.pendingEvents = c.pendingEvents[:0:0] }

func (c *Thread) Reset() {
	for _, event := range slices.Backward(c.pendingEvents) {
		event.undo(c)
	}

	c.ClearEvents()
}

// READ

type ThreadReadOnly interface {
	EventsReader[ThreadEvent]

	ID() ids.ThreadID
	Messages(limit uint) []messages.Message
	AgentID() ids.AgentID
}

func (c *Thread) ID() ids.ThreadID     { return c.id }
func (c *Thread) AgentID() ids.AgentID { return c.agentID }

// Messages returns messages history trimmed to the given limit using
// Sliding Window pattern with Tool-Safety guarantees.
//
// If limit is 0, then all messages will be returned.
//
// Tool-Safety rules:
//  1. Orphaned Responses: If the first message in the window is a tool response/error,
//     it's removed until the first message is MessageUser, MessageAssistant,
//     or MessageToolRequest.
//  2. Incomplete Pairs: If a tool response/error is in the window, but its
//     corresponding request was trimmed, the response is removed.
func (c *Thread) Messages(limit uint) []messages.Message {
	if limit == 0 || uint(len(c.messages)) <= limit {
		return slices.Clone(c.messages)
	}

	//nolint:gosec // safe conversion after length check
	window := c.messages[len(c.messages)-int(limit):]
	result := filterIncompletePairs(slices.Clone(window))

	// 2. Handle Orphaned Responses at the beginning
	for len(result) > 0 {
		if isSafeStart(result[0]) {
			break
		}

		result = result[1:]
	}

	return result
}

// filterIncompletePairs removes orphaned tool responses whose requests were trimmed.
//
// This function MODIFIES the input slice, so it MUST be a clone of the original
// window to avoid side effects on the thread state.
func filterIncompletePairs(window []messages.Message) []messages.Message {
	requests := make(map[string]struct{})

	return slices.DeleteFunc(window, func(msg messages.Message) bool {
		if req, ok := msg.(messages.MessageToolRequest); ok {
			requests[req.ToolCallID()] = struct{}{}

			return false
		}

		if resp, ok := msg.(messages.MessageTool); ok {
			_, hasReq := requests[resp.ToolCallID()]

			return !hasReq
		}

		return false
	})
}

func isSafeStart(m messages.Message) bool {
	switch m.(type) {
	case messages.MessageUser, messages.MessageAssistant, messages.MessageToolRequest:
		return true
	default:
		return false
	}
}

// WRITE

func (c *Thread) AddMessage(message messages.Message) error {
	if ok, err := c.tryMerge(message); ok {
		return err
	}

	msgs := cloneWithAppend(c.messages, message)
	if err := c.validateMessages(msgs); err != nil {
		return err
	}

	c.messages = msgs

	c.pendingEvents = append(c.pendingEvents, ThreadEventMessageAdded{
		message: message,
	})

	return nil
}

func (c *Thread) tryMerge(message messages.Message) (bool, error) {
	if len(c.messages) == 0 {
		return false, nil
	}

	last := c.messages[len(c.messages)-1]
	if last.MergeTag() != message.MergeTag() || last.MergeTag() == 0 {
		return false, nil
	}

	previous := last

	merged, err := messages.MergeMessages(previous, message)
	if err != nil {
		return true, fmt.Errorf("merging messages: %w", err)
	}

	c.messages[len(c.messages)-1] = merged

	c.pendingEvents = append(c.pendingEvents, ThreadEventMessageUpdated{
		message:  merged,
		previous: previous,
	})

	return true, nil
}

func (c *Thread) SetAgent(agentID ids.AgentID) bool {
	previous := c.agentID

	if previous == agentID || !agentID.Valid() {
		return false
	}

	c.agentID = agentID
	c.pendingEvents = append(c.pendingEvents, ThreadEventAgentSet{
		agentID:  agentID,
		previous: previous,
	})

	return true
}

// EVENTS

type ThreadEvent interface{ undo(c *Thread) }

//nolint:exhaustruct // interface check
var _ ThreadEvent = ThreadEventMessageAdded{}

type ThreadEventMessageAdded struct {
	message messages.Message
}

//nolint:ireturn // returns polymorphic message type
func (e ThreadEventMessageAdded) Message() messages.Message { return e.message }

func (e ThreadEventMessageAdded) undo(c *Thread) {
	if n := len(c.messages); n > 0 {
		c.messages[n-1] = nil
		c.messages = c.messages[:n-1]
	}
}

type ThreadEventMessageUpdated struct {
	message  messages.Message
	previous messages.Message
}

func (e ThreadEventMessageUpdated) undo(c *Thread) {
	if n := len(c.messages); n > 0 {
		c.messages[n-1] = e.previous
	}
}

type ThreadEventAgentSet struct {
	agentID  ids.AgentID
	previous ids.AgentID
}

func (e ThreadEventAgentSet) AgentID() ids.AgentID { return e.agentID }

func (e ThreadEventAgentSet) undo(c *Thread) { c.agentID = e.previous }

func cloneWithAppend[S ~[]T, T any](other S, others ...T) S {
	res := make([]T, len(other), len(other)+len(others))
	copy(res, other)
	res = append(res, others...)

	return res
}
