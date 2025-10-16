package entities

import (
	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
)

type Message struct {
	id          ids.MessageID
	replyToID   *ids.MessageID          // id of the message this message is replying to. can be empty
	authorID    ids.UserID              // internal id in our database
	text        *components.MessageText // text of the message. can be empty
	attachments []string                // list of attachment ids

	pendingEvents []MessageEvent
	valid         bool
}

type NewMessageOption func(*Message)

func WithReplyToID(replyToID ids.MessageID) NewMessageOption {
	return func(c *Message) { c.replyToID = &replyToID }
}

func WithText(text components.MessageText) NewMessageOption {
	return func(c *Message) { c.text = &text }
}

func NewMessage(id ids.MessageID, authorID ids.UserID, opts ...NewMessageOption) (*Message, error) {
	c := &Message{
		id:       id,
		authorID: authorID,
	}
	for _, opt := range opts {
		opt(c)
	}

	if err := c.validate(); err != nil {
		return nil, err
	}
	c.valid = true

	return c, nil
}

// VALIDATION

func (c *Message) Valid() bool { return c != nil && (c.valid || c.validate() == nil) }
func (c *Message) validate() error {

	return nil
}

// READ

type MessageReadOnly interface {
	ID() ids.MessageID
	ReplyToID() (ids.MessageID, bool)
	AuthorID() ids.UserID
	Text() (components.MessageText, bool)
}

func (c *Message) ID() ids.MessageID { return c.id }
func (c *Message) ReplyToID() (ids.MessageID, bool) {
	if c.replyToID == nil {
		return ids.MessageID{}, false
	}

	return *c.replyToID, true
}
func (c *Message) AuthorID() ids.UserID { return c.authorID }
func (c *Message) Text() (components.MessageText, bool) {
	if c.text == nil {
		return components.MessageText{}, false
	}

	return *c.text, true
}

// EVENTS

type MessageEvent interface {
	_MessageEvent()
}
