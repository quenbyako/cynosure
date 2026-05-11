package cynosure

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/embedding"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/accounts"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/users"
)

func newChatUsecase(
	ctx context.Context,
	params *appParams,
	storage constructor[ports.ThreadStorageWrapped],
	model chatmodel.PortWrapped,
	tool toolclient.PortWrapped,
	indexer embedding.PortWrapped,
	toolStorage constructor[ports.ToolStorage],
	server constructor[ports.ServerStorage],
	account constructor[ports.AccountStorage],
	models constructor[ports.AgentStorage],
	limiter constructor[ratelimiter.PortWrapped],
) (*chat.Usecase, error) {
	limiterPort, err := limiter.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build limiter port: %w", err)
	}
	accountsPort, err := account.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build accounts port: %w", err)
	}

	serversPort, err := server.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build servers port: %w", err)
	}

	modelsPort, err := models.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build models port: %w", err)
	}

	toolStoragePort, err := toolStorage.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build tool storage port: %w", err)
	}

	storagePort, err := storage.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build storage port: %w", err)
	}

	usecase, err := chat.New(
		storagePort,
		model,
		tool,
		indexer,
		toolStoragePort,
		serversPort,
		accountsPort,
		modelsPort,
		limiterPort,
		chat.WithObservability(params.observability),
		chat.WithChatLimit(params.chat.historyLimit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat usecase: %w", err)
	}

	return usecase, nil
}

func newAccountsUsecase(
	ctx context.Context,
	params *appParams,
	lifecycle *lifecycle,
	servers constructor[ports.ServerStorage],
	oauth oauthhandler.PortWrapped,
	accountsStorage constructor[ports.AccountStorage],
	tools constructor[ports.ToolStorage],
	index embedding.PortWrapped,
	toolClient toolclient.PortWrapped,
	identities identitymanager.PortWrapped,
	limiter constructor[ratelimiter.PortWrapped],
) (*accounts.Usecase, error) {
	limiterPort, err := limiter.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build limiter port: %w", err)
	}

	serversPort, err := servers.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build servers port: %w", err)
	}

	accountsPort, err := accountsStorage.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build accounts port: %w", err)
	}

	toolsPort, err := tools.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build tools port: %w", err)
	}

	usecase, err := accounts.New(
		serversPort,
		oauth,
		accountsPort,
		toolsPort,
		index,
		toolClient,
		identities,
		limiterPort,
		accounts.WithOAuthRedirectURL(params.ory.callback),
		accounts.WithTracerProvider(params.observability),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create accounts usecase: %w", err)
	}

	lifecycle.schedule(usecase.Run)

	return usecase, nil
}

func newUsersUsecase(
	ctx context.Context,
	params *appParams,
	identities identitymanager.PortWrapped,
	agents constructor[ports.AgentStorage],
	accStorage constructor[ports.AccountStorage],
	servers constructor[ports.ServerStorage],
	tools constructor[ports.ToolStorage],
	toolClient toolclient.PortWrapped,
	index embedding.PortWrapped,
	limiter constructor[ratelimiter.PortWrapped],
) (*users.Usecase, error) {
	limiterPort, err := limiter.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build limiter port: %w", err)
	}

	agentsPort, err := agents.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build agents port: %w", err)
	}

	accPort, err := accStorage.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build accounts port: %w", err)
	}

	serversPort, err := servers.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build servers port: %w", err)
	}

	toolsPort, err := tools.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build tools port: %w", err)
	}

	usecase, err := users.New(
		identities,
		agentsPort,
		accPort,
		serversPort,
		toolsPort,
		toolClient,
		index,
		limiterPort,
		params.adminMCPID,
		users.WithTracerProvider(params.observability),
	)
	if err != nil {
		return nil, fmt.Errorf("creating users usecase: %w", err)
	}

	return usecase, nil
}
