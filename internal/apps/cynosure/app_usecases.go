package cynosure

import (
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/accounts"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/users"
)

//nolint:funlen // for now that's okay, maybe later we'll split to 3 usecases.
func newChatUsecase(
	params *appParams,
	storage ports.ThreadStorageWrapped,
	model chatmodel.PortWrapped,
	tool toolclient.PortWrapped,
	indexer ports.ToolSemanticIndex,
	toolStorage ports.ToolStorage,
	server ports.ServerStorage,
	account ports.AccountStorage,
	models ports.AgentStorage,
	limiter ratelimiter.PortWrapped,
	logger chat.LogCallbacks,
) (*chat.Usecase, error) {
	usecase, err := chat.New(
		storage,
		model,
		tool,
		indexer,
		toolStorage,
		server,
		account,
		models,
		limiter,
		chat.WithLogger(logger),
		chat.WithTracer(params.observability),
		chat.WithChatLimit(params.chat.softLimit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat usecase: %w", err)
	}

	return usecase, nil
}

func newAccountsUsecase(
	params *appParams,
	servers ports.ServerStorage,
	oauth oauthhandler.PortWrapped,
	accountsPort ports.AccountStorage,
	tools ports.ToolStorage,
	index ports.ToolSemanticIndex,
	toolClient toolclient.PortWrapped,
	identities identitymanager.PortWrapped,
) (*accounts.Usecase, error) {
	usecase, err := accounts.New(
		servers,
		oauth,
		accountsPort,
		tools,
		index,
		toolClient,
		identities,
		accounts.WithOAuthRedirectURL(params.ory.callback),
		accounts.WithTracerProvider(params.observability),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create accounts usecase: %w", err)
	}

	return usecase, nil
}

func newUsersUsecase(
	params *appParams,
	identities identitymanager.PortWrapped,
	agents ports.AgentStorage,
	accStorage ports.AccountStorage,
	servers ports.ServerStorage,
	tools ports.ToolStorage,
	toolClient toolclient.PortWrapped,
	index ports.ToolSemanticIndex,
) (*users.Usecase, error) {
	usecase, err := users.New(
		identities,
		agents,
		accStorage,
		servers,
		tools,
		toolClient,
		index,
		params.adminMCPID,
		users.WithTracerProvider(params.observability),
	)
	if err != nil {
		return nil, fmt.Errorf("creating users usecase: %w", err)
	}

	return usecase, nil
}
