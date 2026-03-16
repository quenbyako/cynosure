// Package users implements user usecases.
package users

import (
	"errors"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const (
	pkgName = "github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/users"
)

type Usecase struct {
	users      identitymanager.Port
	agents     ports.AgentStorage
	accounts   ports.AccountStorage
	servers    ports.ServerStorage
	tools      ports.ToolStorage
	toolClient toolclient.Port
	index      ports.ToolSemanticIndex
	trace      trace.Tracer
	adminMCPID ids.ServerID
}

type newParams struct {
	tracer trace.TracerProvider
}

type NewOption func(*newParams)

func WithTracerProvider(tp trace.TracerProvider) NewOption {
	return func(p *newParams) { p.tracer = tp }
}

func New(
	users identitymanager.Port,
	agents ports.AgentStorage,
	accounts ports.AccountStorage,
	servers ports.ServerStorage,
	tools ports.ToolStorage,
	toolClient toolclient.Port,
	index ports.ToolSemanticIndex,
	adminMCPID ids.ServerID,
	opts ...NewOption,
) *Usecase {
	params := newParams{
		tracer: noop.NewTracerProvider(),
	}
	for _, opt := range opts {
		opt(&params)
	}

	usecase := &Usecase{
		users:      users,
		agents:     agents,
		accounts:   accounts,
		servers:    servers,
		tools:      tools,
		toolClient: toolClient,
		index:      index,

		adminMCPID: adminMCPID,

		trace: params.tracer.Tracer(pkgName),
	}
	if err := usecase.validate(); err != nil {
		panic(err)
	}

	return usecase
}

func (s *Usecase) validate() error {
	if s.users == nil {
		return errors.New("user storage is required")
	}

	if s.agents == nil {
		return errors.New("agent storage is required")
	}

	if s.accounts == nil {
		return errors.New("account storage is required")
	}

	if s.servers == nil {
		return errors.New("server storage is required")
	}

	if s.tools == nil {
		return errors.New("tool storage is required")
	}

	if s.toolClient == nil {
		return errors.New("tool client is required")
	}

	if s.index == nil {
		return errors.New("tool semantic index is required")
	}

	if !s.adminMCPID.Valid() {
		return errors.New("admin MCP ID is required")
	}

	return nil
}
