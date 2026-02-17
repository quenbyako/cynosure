package chat

import (
	"errors"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const pkgName = "github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"

type Usecase struct {
	storage     ports.ThreadStorage
	model       ports.ChatModel
	tools       ports.ToolClient
	indexer     ports.ToolSemanticIndex
	toolStorage ports.ToolStorage
	servers     ports.ServerStorage
	accounts    ports.AccountStorage
	models      ports.AgentStorage

	defaultModel   ids.AgentID
	agentLoopTurns uint8

	log   LogCallbacks
	trace trace.Tracer
}

type newParams struct {
	log    LogCallbacks
	tracer trace.TracerProvider
}

type NewOpt func(*newParams)

func WithLogger(log LogCallbacks) NewOpt {
	return func(p *newParams) { p.log = log }
}

func WithTracer(tracer trace.TracerProvider) NewOpt {
	return func(p *newParams) { p.tracer = tracer }
}

func New(
	storage ports.ThreadStorage,
	model ports.ChatModel,
	tool ports.ToolClient,
	indexer ports.ToolSemanticIndex,
	toolStorage ports.ToolStorage,
	server ports.ServerStorage,
	account ports.AccountStorage,
	models ports.AgentStorage,
	defaultModel ids.AgentID,
	opts ...NewOpt,
) *Usecase {
	p := newParams{
		log:    NoOpLogCallbacks{},
		tracer: noop.NewTracerProvider(),
	}
	for _, opt := range opts {
		opt(&p)
	}

	u := &Usecase{
		storage:     storage,
		model:       model,
		tools:       tool,
		indexer:     indexer,
		toolStorage: toolStorage,
		servers:     server,
		accounts:    account,
		models:      models,

		agentLoopTurns: 10,
		defaultModel:   defaultModel,

		log:   p.log,
		trace: p.tracer.Tracer(pkgName),
	}
	if err := u.validate(); err != nil {
		panic(err)
	}

	return u
}

func (u *Usecase) validate() error {
	if u.storage == nil {
		return errors.New("storage repository is required")
	}
	if u.model == nil {
		return errors.New("chat model is required")
	}
	if u.tools == nil {
		return errors.New("tool manager is required")
	}
	if u.indexer == nil {
		return errors.New("indexer is required")
	}
	if u.toolStorage == nil {
		return errors.New("tool storage is required")
	}
	if u.servers == nil {
		return errors.New("server storage is required")
	}
	if u.accounts == nil {
		return errors.New("account storage is required")
	}
	if u.models == nil {
		return errors.New("model settings storage is required")
	}
	if !u.defaultModel.Valid() {
		return errors.New("default model is required")
	}

	return nil
}
