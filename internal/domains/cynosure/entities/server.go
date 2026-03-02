package entities

import (
	"fmt"
	"net/url"
	"slices"
	"time"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

type ServerConfig struct {
	id         ids.ServerID
	sseLink    *url.URL
	authConfig *oauth2.Config
	// If expiration of OAuth config is empty — probably config works
	// indefinitely. usually it's temporary when client creates
	// pseudo-application through oauth process.
	configExpiration time.Time

	// default protocol is a type of protocol that vas detected on server.
	// Value MAY be empty (invalid)
	protocol tools.Protocol

	pendingEvents[ServerConfigEvent]
	_valid bool
}

var _ EventsReader[ServerConfigEvent] = (*ServerConfig)(nil)
var _ ServerConfigReadOnly = (*ServerConfig)(nil)

type ServerConfigOption func(*ServerConfig)

func WithExpiration(expiration time.Time) ServerConfigOption {
	return func(c *ServerConfig) { c.configExpiration = expiration }
}

func WithAuthConfig(cfg *oauth2.Config) ServerConfigOption {
	// cloning to avoid external modifications after setting
	return func(c *ServerConfig) { c.authConfig = cloneConfig(cfg) }
}

func WithProtocol(protocol tools.Protocol) ServerConfigOption {
	return func(c *ServerConfig) { c.protocol = protocol }
}

func NewServerConfig(id ids.ServerID, link *url.URL, opts ...ServerConfigOption) (*ServerConfig, error) {
	c := &ServerConfig{
		id:               id,
		sseLink:          link,
		authConfig:       nil,
		configExpiration: time.Time{},
	}
	for _, opt := range opts {
		opt(c)
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}
	c._valid = true

	return c, nil
}

func (c *ServerConfig) Valid() bool { return c._valid || c.Validate() == nil }
func (c *ServerConfig) Validate() error {
	if !c.id.Valid() {
		return fmt.Errorf("invalid server ID")
	}
	if c.sseLink == nil {
		return fmt.Errorf("SSE link is nil")
	}
	if err := c.validateConfig(c.authConfig); err != nil {
		return err
	}

	return nil
}

func (c *ServerConfig) validateConfig(cfg *oauth2.Config) error {
	// TODO: add more validation if necessary
	return nil
}

// CHANGES

func (c *ServerConfig) Synchronized() bool                 { return len(c.pendingEvents) == 0 }
func (c *ServerConfig) PendingEvents() []ServerConfigEvent { return slices.Clone(c.pendingEvents) }
func (c *ServerConfig) ClearEvents()                       { c.pendingEvents = c.pendingEvents[:0:0] }

func (c *ServerConfig) Reset() {
	for _, event := range slices.Backward(c.pendingEvents) {
		event.undo(c)
	}

	c.ClearEvents()
}

// READ

type ServerConfigReadOnly interface {
	EventsReader[ServerConfigEvent]

	ID() ids.ServerID
	SSELink() *url.URL
	AuthConfig() *oauth2.Config
	ConfigExpiration() time.Time
	Protocol() (tools.Protocol, bool)
	PreferredProtocol() tools.Protocol
}

func (c *ServerConfig) ID() ids.ServerID { return c.id }
func (c *ServerConfig) SSELink() *url.URL {
	// cloning to avoid external modifications
	if c.sseLink == nil {
		return nil
	}

	cloned := *c.sseLink
	return &cloned
}

func (c *ServerConfig) AuthConfig() *oauth2.Config { return cloneConfig(c.authConfig) }

func (c *ServerConfig) ConfigExpiration() time.Time { return c.configExpiration }

func (c *ServerConfig) Protocol() (tools.Protocol, bool) {
	return c.protocol, c.protocol.Valid()
}

func (c *ServerConfig) PreferredProtocol() tools.Protocol {
	return c.protocol
}

// WRITE

func (c *ServerConfig) SetOAuthConfig(cfg *oauth2.Config) error {
	if err := c.validateConfig(cfg); err != nil {
		return err
	}
	previous := c.authConfig
	c.authConfig = cloneConfig(cfg)

	c.pendingEvents = append(c.pendingEvents, ServerConfigEventOauthConfigUpdated{
		previous: previous,
		value:    c.authConfig,
	})

	return nil
}

// UpdateSupportedProtocols updates the list of protocols the server supports.
// The order matters: first element is the preferred protocol.
// Valid values: "streamable", "sse"
func (c *ServerConfig) SetProtocol(protocol tools.Protocol) bool {
	if !protocol.Valid() {
		return false
	}

	previous := c.protocol
	c.protocol = protocol

	c.pendingEvents = append(c.pendingEvents, ServerConfigEventProtocolUpdated{
		previous: previous,
		value:    c.protocol,
	})

	return true
}

func (c *ServerConfig) UnsetProcotocol() {
	previous := c.protocol
	c.protocol = tools.Protocol(0) // setting invalid as it's expected to be optional

	c.pendingEvents = append(c.pendingEvents, ServerConfigEventProtocolUpdated{
		previous: previous,
		value:    c.protocol,
	})
}

// EVENTS

type ServerConfigEvent interface{ undo(c *ServerConfig) }

var _ ServerConfigEvent = ServerConfigEventOauthConfigUpdated{}
var _ ServerConfigEvent = ServerConfigEventProtocolUpdated{}

type ServerConfigEventOauthConfigUpdated struct {
	previous *oauth2.Config
	value    *oauth2.Config
}

func (e ServerConfigEventOauthConfigUpdated) Value() *oauth2.Config { return e.value }

func (e ServerConfigEventOauthConfigUpdated) undo(c *ServerConfig) {
	c.authConfig = e.previous
}

type ServerConfigEventProtocolUpdated struct {
	previous tools.Protocol
	value    tools.Protocol
}

func (e ServerConfigEventProtocolUpdated) Value() tools.Protocol { return e.value }

func (e ServerConfigEventProtocolUpdated) undo(c *ServerConfig) {
	c.protocol = e.previous
}

func cloneConfig(cfg *oauth2.Config) *oauth2.Config {
	if cfg == nil {
		return nil
	}

	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:       cfg.Endpoint.AuthURL,
			DeviceAuthURL: cfg.Endpoint.DeviceAuthURL,
			TokenURL:      cfg.Endpoint.TokenURL,
			AuthStyle:     cfg.Endpoint.AuthStyle,
		},
		RedirectURL: cfg.RedirectURL,
		Scopes:      slices.Clone(cfg.Scopes),
	}
}
