// Package chat implements chat usecases.
package chat

import (
	"errors"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
)

const (
	pkgName = "github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
)

type Usecase struct {
	storage        ports.ThreadStorage
	model          chatmodel.Port
	tools          toolclient.Port
	indexer        ports.ToolSemanticIndex
	toolStorage    ports.ToolStorage
	servers        ports.ServerStorage
	accounts       ports.AccountStorage
	models         ports.AgentStorage
	log            LogCallbacks
	trace          trace.Tracer
	agentLoopTurns uint8
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
	model chatmodel.Port,
	tool toolclient.Port,
	indexer ports.ToolSemanticIndex,
	toolStorage ports.ToolStorage,
	server ports.ServerStorage,
	account ports.AccountStorage,
	models ports.AgentStorage,
	opts ...NewOpt,
) *Usecase {
	params := newParams{
		log:    NoOpLogCallbacks{},
		tracer: noop.NewTracerProvider(),
	}
	for _, opt := range opts {
		opt(&params)
	}

	usecase := &Usecase{
		storage:     storage,
		model:       model,
		tools:       tool,
		indexer:     indexer,
		toolStorage: toolStorage,
		servers:     server,
		accounts:    account,
		models:      models,

		agentLoopTurns: 10,

		log:   params.log,
		trace: params.tracer.Tracer(pkgName),
	}
	if err := usecase.validate(); err != nil {
		panic(err)
	}

	return usecase
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

	return nil
}
