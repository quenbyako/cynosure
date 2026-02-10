package entities

import (
	"fmt"
	"slices"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

type Thread struct {
	id       ids.ThreadID
	messages []messages.Message

	pendingEvents[ThreadEvent]
	valid bool
}

var _ EventsReader[ThreadEvent] = (*Thread)(nil)
var _ ThreadReadOnly = (*Thread)(nil)

func NewThread(id ids.ThreadID, messages []messages.Message) (*Thread, error) {
	c := &Thread{
		id:       id,
		messages: messages,
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}
	c.valid = true

	return c, nil
}

func (c *Thread) Valid() bool { return c.valid || c.Validate() == nil }
func (c *Thread) Validate() error {
	if !c.id.Valid() {
		return fmt.Errorf("thread ID is invalid")
	}
	if len(c.messages) == 0 {
		return fmt.Errorf("messages cannot be empty")
	}

	return nil
}

func (c *Thread) validateMessages(messages []messages.Message) error {
	if len(messages) == 0 {
		return fmt.Errorf("messages cannot be empty")
	}
	for i, msg := range messages {
		if !msg.Valid() {
			return fmt.Errorf("message %d is invalid", i)
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
}

func (c *Thread) ID() ids.ThreadID             { return c.id }
func (c *Thread) Messages() []messages.Message { return c.messages }

// WRITE

func (c *Thread) AddMessage(message messages.Message) error {
	messages := append(c.messages, message)
	if err := c.validateMessages(messages); err != nil {
		return err
	}
	c.messages = messages

	c.pendingEvents = append(c.pendingEvents, ThreadEventMessageAdded{
		message: message,
	})

	return nil
}

// EVENTS

type ThreadEvent interface{ undo(c *Thread) }

var _ ThreadEvent = ThreadEventMessageAdded{}

type ThreadEventMessageAdded struct {
	message messages.Message
}

func (e ThreadEventMessageAdded) Message() messages.Message { return e.message }

func (e ThreadEventMessageAdded) undo(c *Thread) {
	if n := len(c.messages); n > 0 {
		c.messages[n-1] = nil
		c.messages = c.messages[:n-1]
	}
}
