// Package users implements user usecases.
package users

import (
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/accounts"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/embedding"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const (
	pkgName = "github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/users"
)

type Usecase struct {
	users      identitymanager.Port
	agents     ports.AgentStorage
	accounts   accounts.Port
	servers    ports.ServerStorage
	tools      ports.ToolStorage
	toolClient toolclient.Port
	index      embedding.Port
	limiter    ratelimiter.Port
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
	users identitymanager.Port, agents ports.AgentStorage, accs accounts.Port,
	servers ports.ServerStorage, tools ports.ToolStorage, toolClient toolclient.Port,
	index embedding.Port, limiter ratelimiter.Port, adminMCPID ids.ServerID,
	opts ...NewOption,
) (*Usecase, error) {
	params := buildNewParams(opts...)

	usecase := &Usecase{
		users: users, agents: agents, accounts: accs, servers: servers,
		tools: tools, toolClient: toolClient, index: index, limiter: limiter,
		adminMCPID: adminMCPID, trace: params.tracer.Tracer(pkgName),
	}

	if err := usecase.validate(); err != nil {
		return nil, err
	}

	return usecase, nil
}

func (u *Usecase) validate() error {
	if err := u.validateStorages(); err != nil {
		return err
	}

	return u.validateClients()
}

func (u *Usecase) validateStorages() error {
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
	default:
		return nil
	}
}

func (u *Usecase) validateClients() error {
	switch {
	case u.toolClient == nil:
		return errInternalValidation("tool client is required")
	case u.index == nil:
		return errInternalValidation("tool semantic index is required")
	case u.limiter == nil:
		return errInternalValidation("rate limiter is required")
	case !u.adminMCPID.Valid():
		return errInternalValidation("admin MCP ID is required")
	default:
		return nil
	}
}
