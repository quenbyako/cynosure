package chat

import (
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
)

type Service struct {
	storage  ports.StorageRepository
	model    ports.ChatModel
	tools    ports.ToolManager
	servers  ports.ServerStorage
	accounts ports.AccountStorage
	models   ports.ModelSettingsStorage

	defaultModel ids.ModelConfigID

	log LogCallbacks
}

func New(
	storage ports.StorageRepository,
	model ports.ChatModel,
	tool ports.ToolManager,
	server ports.ServerStorage,
	account ports.AccountStorage,
	models ports.ModelSettingsStorage,
	defaultModel ids.ModelConfigID,
	log LogCallbacks,
) *Service {
	if storage == nil {
		panic("storage repository is required")
	}
	if model == nil {
		panic("chat model is required")
	}
	if tool == nil {
		panic("tool manager is required")
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

	return &Service{
		storage:  storage,
		model:    model,
		tools:    tool,
		servers:  server,
		accounts: account,
		models:   models,

		defaultModel: defaultModel,

		log: log,
	}
}
