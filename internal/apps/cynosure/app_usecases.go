package cynosure

import (
	"github.com/goforj/wire"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/accounts"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/users"
)

var (
	chatUsecase     = wire.NewSet(newChatUsecase)
	accountsUsecase = wire.NewSet(newAccountsUsecase)
	usersUsecase    = wire.NewSet(newUsersUsecase)
)

func newChatUsecase(
	p *appParams,
	storage ports.ThreadStorageWrapped,
	model ports.ChatModel,
	tool toolclient.PortWrapped,
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
	oauth oauthhandler.PortWrapped,
	accountsPort ports.AccountStorage,
	tools ports.ToolStorage,
	index ports.ToolSemanticIndex,
	toolClient toolclient.PortWrapped,
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
		accounts.WithOAuthRedirectURL(p.oauthCallback),
		accounts.WithTracerProvider(p.observability),
	))
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
