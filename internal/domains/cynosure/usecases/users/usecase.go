// Package users implements user usecases.
package users

import (
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

func buildNewParams(opts ...NewOption) newParams {
	params := newParams{
		tracer: noop.NewTracerProvider(),
	}

	for _, opt := range opts {
		opt(&params)
	}

	return params
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
) (*Usecase, error) {
	params := buildNewParams(opts...)

	usecase := &Usecase{
		users:      users,
		agents:     agents,
		accounts:   accounts,
		servers:    servers,
		tools:      tools,
		toolClient: toolClient,
		index:      index,
		adminMCPID: adminMCPID,
		trace:      params.tracer.Tracer(pkgName),
	}

	if err := usecase.validate(); err != nil {
		return nil, err
	}

	return usecase, nil
}

func (u *Usecase) validate() error {
	switch {
	case u.users == nil:
		return errInternalValidation("user storage is required")
	case u.agents == nil:
		return errInternalValidation("agent storage is required")
	case u.accounts == nil:
		return errInternalValidation("account storage is required")
	case u.servers == nil:
		return errInternalValidation("server storage is required")
	case u.tools == nil:
		return errInternalValidation("tool storage is required")
	case u.toolClient == nil:
		return errInternalValidation("tool client is required")
	case u.index == nil:
		return errInternalValidation("tool semantic index is required")
	case !u.adminMCPID.Valid():
		return errInternalValidation("admin MCP ID is required")
	default:
		return nil
	}
}
