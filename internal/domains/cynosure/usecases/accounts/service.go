package accounts

import (
	"crypto/rand"
	"errors"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

const pkgName = "github.com/quenbyako/cynosure/internal/domains/cynosure/services/accounts"

var (
	ErrAuthUnsupported = errors.New("authorization for this server is not supported, allowed to connect anonymously")
)

type Service struct {
	oauth   ports.OAuthHandler
	servers ports.ServerStorage
	tools   ports.ToolManager
	clock   func() time.Time

	oauthClientName string
	key             [16]byte
	stateExpiration time.Duration

	trace trace.Tracer
}

type newParams struct {
	clientName      string
	fixedKey        [16]byte
	stateExpiration time.Duration
	tracer          trace.TracerProvider
}

type NewOption func(*newParams)

func WithOAuthClientName(name string) NewOption {
	return func(p *newParams) { p.clientName = name }
}

func WithFixedKey(key [16]byte) NewOption {
	return func(p *newParams) { p.fixedKey = key }
}

func WithStateExpiration(d time.Duration) NewOption {
	return func(p *newParams) { p.stateExpiration = d }
}

func WithTracerProvider(tp trace.TracerProvider) NewOption {
	return func(p *newParams) { p.tracer = tp }
}

func New(servers ports.ServerStorage, oauth ports.OAuthHandler, tools ports.ToolManager, opts ...NewOption) *Service {
	p := newParams{
		clientName:      "test-client",
		fixedKey:        randomAuthKey(),
		stateExpiration: 5 * time.Minute,
		tracer:          trace.NewNoopTracerProvider(),
	}
	for _, opt := range opts {
		opt(&p)
	}

	s := &Service{
		tools:   tools,
		oauth:   oauth,
		servers: servers,
		clock:   time.Now,

		oauthClientName: p.clientName,
		key:             p.fixedKey,
		stateExpiration: p.stateExpiration,
		trace:           p.tracer.Tracer(pkgName),
	}
	if err := s.validate(); err != nil {
		panic(err)
	}

	return s
}

func (s *Service) validate() error {
	switch {
	case s.tools == nil:
		return errors.New("tool manager is required")
	case s.servers == nil:
		return errors.New("server storage is required")
	case s.oauth == nil:
		return errors.New("OAuth handler is required")
	case s.oauthClientName == "":
		return errors.New("OAuth client name is required")
	case s.key == [16]byte{}:
		return errors.New("OAuth key is required")
	case s.stateExpiration == 0:
		return errors.New("state expiration is required")
	default:
		return nil
	}
}

func randomAuthKey() [16]byte {
	var key [16]byte

	if _, err := rand.Read(key[:]); err != nil {
		panic(err)
	}
	return key
}
