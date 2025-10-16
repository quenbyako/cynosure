package servers

import (
	"errors"
	"net/url"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

const pkgName = "github.com/quenbyako/cynosure/internal/domains/cynosure/services/servers"

type Service struct {
	oauth   ports.OAuthHandler
	servers ports.ServerStorage
	clock   func() time.Time

	oauthClientName string
	authRedirectURL *url.URL
	trace           trace.Tracer
}

type newParams struct {
	clientName string
	trace      trace.TracerProvider
}

type NewOption func(*newParams)

func WithOAuthClientName(name string) NewOption {
	return func(p *newParams) { p.clientName = name }
}

func WithTracerProvider(provider trace.TracerProvider) NewOption {
	return func(p *newParams) { p.trace = provider }
}

func New(servers ports.ServerStorage, oauth ports.OAuthHandler, redirectLink *url.URL, opts ...NewOption) *Service {
	params := newParams{
		clientName: "test-client",
		trace:      noop.NewTracerProvider(),
	}
	for _, opt := range opts {
		opt(&params)
	}

	s := &Service{
		oauth:   oauth,
		servers: servers,
		clock:   time.Now,

		oauthClientName: params.clientName,
		authRedirectURL: redirectLink,
		trace:           params.trace.Tracer(pkgName),
	}
	if err := s.validate(); err != nil {
		panic(err)
	}

	return s
}

func (s *Service) validate() error {
	switch {
	case s.servers == nil:
		return errors.New("server storage is required")
	case s.oauth == nil:
		return errors.New("OAuth handler is required")
	case s.oauthClientName == "":
		return errors.New("OAuth client name is required")
	case s.authRedirectURL == nil:
		return errors.New("auth redirect URL is required")
	default:
		return nil
	}
}
