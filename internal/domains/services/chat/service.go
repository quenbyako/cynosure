package chat

import (
	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/ports"
)

type Service struct {
	storage  ports.StorageRepository
	model    ports.ChatModel
	tools    ports.ToolManager
	servers  ports.ServerStorage
	accounts ports.AccountStorage
	models   ports.ModelSettingsStorage

	defaultModel ids.ModelConfigID
}

func New(
	storage ports.StorageRepository,
	model ports.ChatModel,
	tool ports.ToolManager,
	server ports.ServerStorage,
	account ports.AccountStorage,
	models ports.ModelSettingsStorage,
	defaultModel ids.ModelConfigID,
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
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
