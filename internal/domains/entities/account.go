package entities

import (
	"fmt"
	"slices"

	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/components/tools"

	"golang.org/x/oauth2"
)

type Account struct {
	id          ids.AccountID
	name        string
	description string
	token       *oauth2.Token
	tools       []tools.ToolInfo

	pendingEvents[AccountEvent]
	valid bool
}

var _ EventsReader[AccountEvent] = (*Account)(nil)
var _ AccountReadOnly = (*Account)(nil)

type NewAccountOption func(*Account)

func WithAuthToken(token *oauth2.Token) NewAccountOption {
	return func(c *Account) { c.token = token }
}

func NewAccount(id ids.AccountID, name, description string, tools []tools.ToolInfo, opts ...NewAccountOption) (*Account, error) {
	c := &Account{
		id:          id,
		name:        name,
		description: description,
		token:       nil,
		tools:       tools,
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

func (c *Account) Valid() bool { return c != nil && (c.valid || c.validate() == nil) }
func (c *Account) validate() error {
	if c.name == "" {
		return fmt.Errorf("name is required")
	}
	if c.description == "" {
		return fmt.Errorf("description is required")
	}

	if err := c.validateTools(c.tools); err != nil {
		return err
	}

	return nil
}

func (*Account) validateTools(tools []tools.ToolInfo) error {
	seen := make(map[string]struct{}, len(tools))
	for i, t := range tools {
		if !t.Valid() {
			return fmt.Errorf("invalid tool %d: %w", i, t.Validate())
		}

		if _, ok := seen[t.Name()]; ok {
			return fmt.Errorf("duplicated tool name: %q", t.Name())
		}

		seen[t.Name()] = struct{}{}
	}

	return nil
}

// CHANGES

func (c *Account) Synchronized() bool            { return len(c.pendingEvents) == 0 }
func (c *Account) PendingEvents() []AccountEvent { return slices.Clone(c.pendingEvents) }
func (c *Account) ClearEvents()                  { c.pendingEvents = c.pendingEvents[:0] }

// READ

type AccountReadOnly interface {
	ID() ids.AccountID
	Token() *oauth2.Token
	Name() string
	Description() string
	Tools() []tools.ToolInfo
}

func (c *Account) ID() ids.AccountID       { return c.id }
func (c *Account) Token() *oauth2.Token    { return c.token }
func (c *Account) Name() string            { return c.name }
func (c *Account) Description() string     { return c.description }
func (c *Account) Tools() []tools.ToolInfo { return slices.Clone(c.tools) }

// WRITE

func (c *Account) SetTools(tools []tools.ToolInfo) error {
	if err := c.validateTools(tools); err != nil {
		return err
	}

	c.tools = tools
	c.pendingEvents = append(c.pendingEvents, AccountEventToolsSetted{
		tools: tools,
	})

	return nil
}

func (c *Account) UpdateToken(token *oauth2.Token) error {
	if token == nil {
		if c.token == nil { // accounts MAY be anonymous, that's okay
			return nil
		}

		return fmt.Errorf("token cannot be nil")
	}
	if !token.Valid() {
		return fmt.Errorf("invalid token")
	}

	c.token = token
	c.pendingEvents = append(c.pendingEvents, AccountEventTokenUpdated{
		token: token,
	})

	return nil
}

// EVENTS

type AccountEvent interface {
	_AccountEvent()
}

var _ AccountEvent = AccountEventToolsSetted{}

type AccountEventToolsSetted struct {
	tools []tools.ToolInfo
}

func (e AccountEventToolsSetted) _AccountEvent() {}

func (e AccountEventToolsSetted) Tools() []tools.ToolInfo { return e.tools }

type AccountEventTokenUpdated struct {
	token *oauth2.Token
}

func (e AccountEventTokenUpdated) _AccountEvent() {}

func (e AccountEventTokenUpdated) Token() *oauth2.Token { return e.token }
