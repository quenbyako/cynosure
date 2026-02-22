package users

import (
	"errors"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

const (
	pkgName = "github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/users"
)

type Usecase struct {
	users  ports.IdentityManager
	agents ports.AgentStorage

	trace trace.Tracer
}

type newParams struct {
	tracer trace.TracerProvider
}

type NewOption func(*newParams)

func WithTracerProvider(tp trace.TracerProvider) NewOption {
	return func(p *newParams) { p.tracer = tp }
}

func New(users ports.IdentityManager, agents ports.AgentStorage, opts ...NewOption) *Usecase {
	p := newParams{
		tracer: noop.NewTracerProvider(),
	}
	for _, opt := range opts {
		opt(&p)
	}

	s := &Usecase{
		users:  users,
		agents: agents,

		trace: p.tracer.Tracer(pkgName),
	}
	if err := s.validate(); err != nil {
		panic(err)
	}

	return s
}

func (s *Usecase) validate() error {
	if s.users == nil {
		return errors.New("user storage is required")
	}
	if s.agents == nil {
		return errors.New("agent storage is required")
	}

	return nil
}
