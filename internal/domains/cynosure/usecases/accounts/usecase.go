package accounts

import (
	"crypto/rand"
	"errors"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

const pkgName = "github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/accounts"

var (
	ErrAuthUnsupported = errors.New("authorization for this server is not supported, allowed to connect anonymously")
)

type Usecase struct {
	oauth      ports.OAuthHandler
	servers    ports.ServerStorage
	accounts   ports.AccountStorage
	tools      ports.ToolStorage
	index      ports.ToolSemanticIndex
	toolClient ports.ToolClient
	users      ports.IdentityManager
	clock      func() time.Time

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

func New(
	servers ports.ServerStorage,
	oauth ports.OAuthHandler,
	accounts ports.AccountStorage,
	tools ports.ToolStorage,
	index ports.ToolSemanticIndex,
	toolClient ports.ToolClient,
	users ports.IdentityManager,
	opts ...NewOption,
) (
	*Usecase,
	error,
) {
	p := newParams{
		clientName:      "test-client",
		fixedKey:        randomAuthKey(),
		stateExpiration: 5 * time.Minute,
		tracer:          noop.NewTracerProvider(),
	}
	for _, opt := range opts {
		opt(&p)
	}

	s := &Usecase{
		toolClient: toolClient,
		oauth:      oauth,
		servers:    servers,
		accounts:   accounts,
		tools:      tools,
		index:      index,
		users:      users,
		clock:      time.Now,

		oauthClientName: p.clientName,
		key:             p.fixedKey,
		stateExpiration: p.stateExpiration,

		trace: p.tracer.Tracer(pkgName),
	}
	if err := s.validate(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Usecase) validate() error {
	if s.toolClient == nil {
		return errors.New("tool registry is required")
	}
	if s.servers == nil {
		return errors.New("server storage is required")
	}
	if s.oauth == nil {
		return errors.New("OAuth handler is required")
	}
	if s.accounts == nil {
		return errors.New("account storage is required")
	}
	if s.tools == nil {
		return errors.New("tool storage is required")
	}
	if s.index == nil {
		return errors.New("tool semantic index is required")
	}
	if s.users == nil {
		return errors.New("user storage is required")
	}
	if s.oauthClientName == "" {
		return errors.New("OAuth client name is required")
	}
	if s.key == [16]byte{} {
		return errors.New("OAuth key is required")
	}
	if s.stateExpiration == 0 {
		return errors.New("state expiration is required")
	}

	return nil
}

func randomAuthKey() [16]byte {
	var key [16]byte

	if _, err := rand.Read(key[:]); err != nil {
		panic(err)
	}
	return key
}
