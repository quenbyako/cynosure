package entities

import (
	"fmt"
	"slices"

	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/components/messages"
)

type ChatHistory struct {
	user     ids.UserID
	threadID string
	messages []messages.Message

	pendingEvents[ChatHistoryEvent]
	valid bool
}

var _ EventsReader[ChatHistoryEvent] = (*ChatHistory)(nil)
var _ ChatHistoryReadOnly = (*ChatHistory)(nil)

func NewChatHistory(user ids.UserID, threadID string, messages []messages.Message) (*ChatHistory, error) {
	c := &ChatHistory{
		user:     user,
		threadID: threadID,
		messages: messages,
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}
	c.valid = true

	return c, nil
}

func (c *ChatHistory) Valid() bool { return c.valid || c.Validate() == nil }
func (c *ChatHistory) Validate() error {
	if !c.user.Valid() {
		return fmt.Errorf("user ID is invalid")
	}
	if c.threadID == "" {
		return fmt.Errorf("thread ID cannot be empty")
	}
	if len(c.messages) == 0 {
		return fmt.Errorf("messages cannot be empty")
	}

	return nil
}

func (c *ChatHistory) validateMessages(messages []messages.Message) error {
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

func (c *ChatHistory) Synchronized() bool                { return len(c.pendingEvents) == 0 }
func (c *ChatHistory) PendingEvents() []ChatHistoryEvent { return slices.Clone(c.pendingEvents) }
func (c *ChatHistory) ClearEvents()                      { c.pendingEvents = c.pendingEvents[:0:0] }

func (c *ChatHistory) Reset() {
	for _, event := range slices.Backward(c.pendingEvents) {
		event.undo(c)
	}

	c.ClearEvents()
}

// READ

type ChatHistoryReadOnly interface {
	EventsReader[ChatHistoryEvent]

	Messages() []messages.Message
	User() ids.UserID
	ThreadID() string
}

func (c *ChatHistory) Messages() []messages.Message { return c.messages }
func (c *ChatHistory) User() ids.UserID             { return c.user }
func (c *ChatHistory) ThreadID() string             { return c.threadID }

// WRITE

func (c *ChatHistory) AddMessage(message messages.Message) error {
	messages := append(c.messages, message)
	if err := c.validateMessages(messages); err != nil {
		return err
	}
	c.messages = messages

	c.pendingEvents = append(c.pendingEvents, ChatHistoryEventMessageAdded{
		message: message,
	})

	return nil
}

// EVENTS

type ChatHistoryEvent interface{ undo(c *ChatHistory) }

var _ ChatHistoryEvent = ChatHistoryEventMessageAdded{}

type ChatHistoryEventMessageAdded struct {
	message messages.Message
}

func (e ChatHistoryEventMessageAdded) Message() messages.Message { return e.message }

func (e ChatHistoryEventMessageAdded) undo(c *ChatHistory) {
	if n := len(c.messages); n > 0 {
		c.messages[n-1] = nil
		c.messages = c.messages[:n-1]
	}
}
