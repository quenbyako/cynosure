package chat

import (
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
	log LogCallbacks,
) *Usecase {
	if storage == nil {
		panic("storage repository is required")
	}
	if model == nil {
		panic("chat model is required")
	}
	if tool == nil {
		panic("tool manager is required")
	}
	if indexer == nil {
		panic("indexer is required")
	}
	if toolStorage == nil {
		panic("tool storage is required")
	}
	if server == nil {
		panic("server storage is required")
	}
	if account == nil {
		panic("account storage is required")
	}
	if models == nil {
		panic("model settings storage is required")
	}
	if !defaultModel.Valid() {
		panic("default model is required")
	}

	return &Usecase{
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

		log:   log,
		trace: noop.NewTracerProvider().Tracer(pkgName),
	}
}
