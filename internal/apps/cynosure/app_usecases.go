package cynosure

import (
	"github.com/goforj/wire"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/accounts"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/servers"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/users"
)

var (
	chatUsecase     = wire.NewSet(newChatUsecase)
	accountsUsecase = wire.NewSet(newAccountsUsecase)
	serversUsecase  = wire.NewSet(newServersUsecase)
	usersUsecase    = wire.NewSet(newUsersUsecase)
)

func newChatUsecase(
	p *appParams,
	storage ports.ThreadStorage,
	model ports.ChatModel,
	tool ports.ToolClient,
	indexer ports.ToolSemanticIndex,
	toolStorage ports.ToolStorage,
	server ports.ServerStorage,
	account ports.AccountStorage,
	models ports.AgentStorage,
	logger chat.LogCallbacks,
) *chat.Usecase {
	defaultModelConfig := must(ids.NewAgentIDFromString(p.defaultModelConfig))

	return chat.New(
		storage,
		model,
		tool,
		indexer,
		toolStorage,
		server,
		account,
		models,
		defaultModelConfig,
		chat.WithLogger(logger),
		chat.WithTracer(p.observability),
	)
}

func newAccountsUsecase(
	p *appParams,
	servers ports.ServerStorage,
	oauth ports.OAuthHandler,
	accountsPort ports.AccountStorage,
	tools ports.ToolStorage,
	index ports.ToolSemanticIndex,
	toolClient ports.ToolClient,
	users ports.UserStorage,
) *accounts.Usecase {
	return accounts.New(
		servers,
		oauth,
		accountsPort,
		tools,
		index,
		toolClient,
		users,
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

func newUsersUsecase(
	p *appParams,
	usersPort ports.UserStorage,
	agents ports.AgentStorage,
) *users.Usecase {
	return users.New(
		usersPort,
		agents,
		users.WithTracerProvider(p.observability),
	)
}
