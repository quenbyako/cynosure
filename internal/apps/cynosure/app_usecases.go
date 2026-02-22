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
	storage ports.ThreadStorageWrapped,
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
	identities ports.IdentityManagerWrapped,
) *accounts.Usecase {
	return must(accounts.New(
		servers,
		oauth,
		accountsPort,
		tools,
		index,
		toolClient,
		identities,
		accounts.WithTracerProvider(p.observability),
	))
}

func newServersUsecase(
	p *appParams,
	storage ports.ServerStorage,
	oauth ports.OAuthHandler,
	toolClient ports.ToolClient,
) *servers.Usecase {
	return servers.New(storage, oauth, toolClient, p.oauthCallback,
		servers.WithTracerProvider(p.observability),
	)
}

func newUsersUsecase(
	p *appParams,
	identities ports.IdentityManagerWrapped,
	agents ports.AgentStorage,
) *users.Usecase {
	return users.New(
		identities,
		agents,
		users.WithTracerProvider(p.observability),
	)
}
