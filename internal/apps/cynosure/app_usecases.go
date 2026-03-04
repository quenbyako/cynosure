package cynosure

import (
	"github.com/goforj/wire"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
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
	return chat.New(
		storage,
		model,
		tool,
		indexer,
		toolStorage,
		server,
		account,
		models,
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
	identities identitymanager.PortWrapped,
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
	identities identitymanager.PortWrapped,
	agents ports.AgentStorage,
	accounts ports.AccountStorage,
	servers ports.ServerStorage,
	tools ports.ToolStorage,
	toolClient toolclient.PortWrapped,
	index ports.ToolSemanticIndex,
) *users.Usecase {
	return users.New(
		identities,
		agents,
		accounts,
		servers,
		tools,
		toolClient,
		index,
		p.adminMCPID,
		users.WithTracerProvider(p.observability),
	)
}
