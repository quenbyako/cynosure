package cynosure

import (
	"github.com/google/wire"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/accounts"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/servers"
)

var (
	chatUsecase     = wire.NewSet(newChatUsecase)
	accountsUsecase = wire.NewSet(newAccountsUsecase)
	serversUsecase  = wire.NewSet(newServersUsecase)
)

func newChatUsecase(
	p *appParams,
	storage ports.StorageRepository,
	model ports.ChatModel,
	tool ports.ToolManager,
	server ports.ServerStorage,
	account ports.AccountStorage,
	models ports.ModelSettingsStorage,
	logger chat.LogCallbacks,
) *chat.Service {
	defaultModelConfig := must(ids.NewModelConfigIDFromString(p.defaultModelConfig))

	return chat.New(
		storage,
		model,
		tool,
		server,
		account,
		models,
		defaultModelConfig,
		logger,
	)
}

func newAccountsUsecase(
	p *appParams,
	storage ports.ServerStorage,
	oauth ports.OAuthHandler,
	tool ports.ToolManager,
) *accounts.Service {
	return accounts.New(storage, oauth, tool,
		accounts.WithTracerProvider(p.observability),
	)
}

func newServersUsecase(
	p *appParams,
	storage ports.ServerStorage,
	oauth ports.OAuthHandler,
) *servers.Service {
	return servers.New(storage, oauth, p.oauthCallback,
		servers.WithTracerProvider(p.observability),
	)
}
