package entities

import (
	"errors"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// Account represents an OAuth2 account with associated MCP server. It holds
// cache of received tools, if any. In addition, it provides methods to refresh
// the token and update instructions for this tools set.
type Account struct {
	token       *oauth2.Token
	name        string
	description string
	pendingEvents[AccountEvent]
	id     ids.AccountID
	_valid bool
}

var (
	_ EventsReader[AccountEvent] = (*Account)(nil)
	_ AccountReadOnly            = (*Account)(nil)
)

type NewAccountOption func(*Account)

func WithAuthToken(token *oauth2.Token) NewAccountOption {
	return func(c *Account) { c.token = token }
}

func NewAccount(id ids.AccountID, name, description string, opts ...NewAccountOption) (*Account, error) {
	account := &Account{
		id:          id,
		name:        name,
		description: description,
		token:       nil,

		pendingEvents: pendingEvents[AccountEvent]{},
		_valid:        false,
	}
	for _, opt := range opts {
		opt(account)
	}

	if err := account.validate(); err != nil {
		return nil, err
	}

	account._valid = true

	return account, nil
}

// VALIDATION

func (c *Account) Valid() bool { return c != nil && (c._valid || c.validate() == nil) }

func (c *Account) validate() error {
	if c.name == "" {
		return errors.New("name is required")
	}

	if c.description == "" {
		return errors.New("description is required")
	}

	return nil
}

// READ

type AccountReadOnly interface {
	ID() ids.AccountID
	Token() *oauth2.Token
	Name() string
	Description() string
}

func (c *Account) ID() ids.AccountID    { return c.id }
func (c *Account) Token() *oauth2.Token { return c.token }
func (c *Account) Name() string         { return c.name }
func (c *Account) Description() string  { return c.description }

// WRITE

func (c *Account) UpdateToken(token *oauth2.Token) error {
	if token == nil {
		if c.token == nil { // accounts MAY be anonymous, that's okay
			return nil
		}

		return errors.New("token cannot be nil")
	}

	if !token.Valid() {
		return errors.New("invalid token")
	}

	c.token = token
	c.pendingEvents = append(c.pendingEvents, AccountEventTokenUpdated{
		token: token,
	})

	return nil
}

// EVENTS

// AccountEvent is an unify interface to aggregate all event types related to
// [Account] modifications.
//
// Available types:
//
//   - [AccountEventTokenUpdated]
type AccountEvent interface {
	_AccountEvent()
}

type AccountEventTokenUpdated struct {
	token *oauth2.Token
}

func (e AccountEventTokenUpdated) _AccountEvent() {}

func (e AccountEventTokenUpdated) Token() *oauth2.Token { return e.token }
