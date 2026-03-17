package entities

import (
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
	messages []messages.Message,
	opts ...ThreadOption,
) (*Thread, error) {
	thread := &Thread{
		id:            id,
		messages:      messages,
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

func (c *Thread) validateMessages(messages []messages.Message) error {
	if len(messages) == 0 {
		return ErrInternalValidation("messages cannot be empty")
	}

	for i, msg := range messages {
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
	Messages() []messages.Message
	AgentID() ids.AgentID
}

func (c *Thread) ID() ids.ThreadID             { return c.id }
func (c *Thread) AgentID() ids.AgentID         { return c.agentID }
func (c *Thread) Messages() []messages.Message { return c.messages }

// WRITE

func (c *Thread) AddMessage(message messages.Message) error {
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
