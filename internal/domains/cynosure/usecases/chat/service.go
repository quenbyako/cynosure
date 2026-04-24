// Package chat implements chat usecases.
package chat

import (
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
)

const (
	pkgName = "github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"

	defaultAgentLoopTurns = 10
	defaultChatLimit      = 20
)

type Usecase struct {
	storage          ports.ThreadStorage
	model            chatmodel.Port
	tools            toolclient.Port
	indexer          ports.ToolSemanticIndex
	toolStorage      ports.ToolStorage
	servers          ports.ServerStorage
	accounts         ports.AccountStorage
	agents           ports.AgentStorage
	limiter          ratelimiter.Port
	log              LogCallbacks
	trace            trace.Tracer
	agentLoopTurns   uint8
	defaultChatLimit uint
}

type newParams struct {
	log       LogCallbacks
	tracer    trace.TracerProvider
	chatLimit uint
}

func buildNewParams(opts ...NewOpt) *newParams {
	params := newParams{
		log:       NoOpLogCallbacks{},
		tracer:    noop.NewTracerProvider(),
		chatLimit: defaultChatLimit,
	}

	for _, opt := range opts {
		opt(&params)
	}

	return &params
}

type NewOpt func(*newParams)

func WithLogger(log LogCallbacks) NewOpt {
	return func(p *newParams) { p.log = log }
}

func WithTracer(tracer trace.TracerProvider) NewOpt {
	return func(p *newParams) { p.tracer = tracer }
}

func WithChatLimit(limit uint) NewOpt {
	return func(p *newParams) { p.chatLimit = limit }
}

// New creates a new usecase instance.
//
// TODO: find a way, how to reduce amount of ports in usecases.
//
//nolint:funlen // it's impossible for now to reduce size, cause there are too many required ports
func New(
	storage ports.ThreadStorage,
	model chatmodel.Port,
	tool toolclient.Port,
	indexer ports.ToolSemanticIndex,
	toolStorage ports.ToolStorage,
	server ports.ServerStorage,
	account ports.AccountStorage,
	models ports.AgentStorage,
	limiter ratelimiter.Port,
	opts ...NewOpt,
) (*Usecase, error) {
	params := buildNewParams(opts...)
	usecase := &Usecase{
		storage:          storage,
		model:            model,
		tools:            tool,
		indexer:          indexer,
		toolStorage:      toolStorage,
		servers:          server,
		accounts:         account,
		agents:           models,
		limiter:          limiter,
		agentLoopTurns:   defaultAgentLoopTurns,
		defaultChatLimit: params.chatLimit,
		log:              params.log,
		trace:            params.tracer.Tracer(pkgName),
	}

	if err := usecase.validate(); err != nil {
		return nil, err
	}

	return usecase, nil
}

//nolint:cyclop // it's just a bunch of nil checks
func (u *Usecase) validate() error {
	switch {
	case u.storage == nil:
		return errInternalValidation("storage repository is required")
	case u.model == nil:
		return errInternalValidation("chat model is required")
	case u.tools == nil:
		return errInternalValidation("tool manager is required")
	case u.indexer == nil:
		return errInternalValidation("indexer is required")
	case u.toolStorage == nil:
		return errInternalValidation("tool storage is required")
	case u.servers == nil:
		return errInternalValidation("server storage is required")
	case u.accounts == nil:
		return errInternalValidation("account storage is required")
	case u.agents == nil:
		return errInternalValidation("model settings storage is required")
	case u.limiter == nil:
		return errInternalValidation("rate limiter is required")
	default:
		return nil
	}
}
