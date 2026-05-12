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
	dps, err := buildChatPorts(ctx, account, server, models, toolStorage, storage)
	if err != nil {
		return nil, err
	}

	return buildChat(ctx, params, model, tool, indexer, limiter, dps)
}

func buildChat(
	ctx context.Context,
	params *appParams,
	model chatmodel.PortWrapped,
	tool toolclient.PortWrapped,
	indexer embedding.PortWrapped,
	limiter constructor[ratelimiter.PortWrapped],
	dps chatPorts,
) (*chat.Usecase, error) {
	lim, err := limiter.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build limiter port: %w", err)
	}

	usecase, err := chat.New(
		dps.thread, model, tool, indexer, dps.tools, dps.server, dps.account, dps.agents, lim,
		chat.WithObservability(params.observability),
		chat.WithChatLimit(params.chat.historyLimit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat usecase: %w", err)
	}

	return usecase, nil
}

type chatPorts struct {
	account ports.AccountStorage
	server  ports.ServerStorage
	agents  ports.AgentStorage
	tools   ports.ToolStorage
	thread  ports.ThreadStorageWrapped
}

func buildChatPorts(
	ctx context.Context,
	account constructor[ports.AccountStorage],
	server constructor[ports.ServerStorage],
	models constructor[ports.AgentStorage],
	toolStorage constructor[ports.ToolStorage],
	storage constructor[ports.ThreadStorageWrapped],
) (chatPorts, error) {
	accs, err1 := account.Build(ctx)
	srvs, err2 := server.Build(ctx)
	ags, err3 := models.Build(ctx)
	tStr, err4 := toolStorage.Build(ctx)
	str, err5 := storage.Build(ctx)

	if err := firstErr(err1, err2, err3, err4, err5); err != nil {
		return chatPorts{}, err
	}

	return chatPorts{account: accs, server: srvs, agents: ags, tools: tStr, thread: str}, nil
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
	dps, err := buildAccountsPorts(ctx, servers, accountsStorage, tools)
	if err != nil {
		return nil, err
	}

	return buildAccounts(ctx, params, lifecycle, oauth, index, toolClient, identities, limiter, dps)
}

func buildAccounts(
	ctx context.Context,
	params *appParams,
	lifecycle *lifecycle,
	oauth oauthhandler.PortWrapped,
	index embedding.PortWrapped,
	toolClient toolclient.PortWrapped,
	identities identitymanager.PortWrapped,
	limiter constructor[ratelimiter.PortWrapped],
	dps accountsPorts,
) (*accounts.Usecase, error) {
	lim, err := limiter.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build limiter port: %w", err)
	}

	usecase, err := accounts.New(
		dps.server, oauth, dps.account, dps.tools, index, toolClient,
		identities, lim,
		accounts.WithOAuthRedirectURL(params.ory.callback),
		accounts.WithTracerProvider(params.observability),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create accounts usecase: %w", err)
	}

	lifecycle.schedule(usecase.Run)

	return usecase, nil
}

type accountsPorts struct {
	server  ports.ServerStorage
	account ports.AccountStorage
	tools   ports.ToolStorage
}

func buildAccountsPorts(
	ctx context.Context,
	servers constructor[ports.ServerStorage],
	accountsStorage constructor[ports.AccountStorage],
	tools constructor[ports.ToolStorage],
) (accountsPorts, error) {
	srvs, err1 := servers.Build(ctx)
	accs, err2 := accountsStorage.Build(ctx)
	tStr, err3 := tools.Build(ctx)

	if err := firstErr(err1, err2, err3); err != nil {
		return accountsPorts{}, err
	}

	return accountsPorts{server: srvs, account: accs, tools: tStr}, nil
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
	dps, err := buildUsersPorts(ctx, agents, accStorage, servers, tools)
	if err != nil {
		return nil, err
	}

	return buildUsers(ctx, params, identities, toolClient, index, limiter, dps)
}

func buildUsers(
	ctx context.Context,
	params *appParams,
	identities identitymanager.PortWrapped,
	toolClient toolclient.PortWrapped,
	index embedding.PortWrapped,
	limiter constructor[ratelimiter.PortWrapped],
	dps usersPorts,
) (*users.Usecase, error) {
	lim, err := limiter.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build limiter port: %w", err)
	}

	usecase, err := users.New(
		identities, dps.agents, dps.account, dps.server,
		dps.tools, toolClient, index, lim,
		params.adminMCPID,
		users.WithTracerProvider(params.observability),
	)
	if err != nil {
		return nil, fmt.Errorf("creating users usecase: %w", err)
	}

	return usecase, nil
}

type usersPorts struct {
	agents  ports.AgentStorage
	account ports.AccountStorage
	server  ports.ServerStorage
	tools   ports.ToolStorage
}

func buildUsersPorts(
	ctx context.Context,
	agents constructor[ports.AgentStorage],
	accStorage constructor[ports.AccountStorage],
	servers constructor[ports.ServerStorage],
	tools constructor[ports.ToolStorage],
) (usersPorts, error) {
	ags, err1 := agents.Build(ctx)
	accs, err2 := accStorage.Build(ctx)
	srvs, err3 := servers.Build(ctx)
	tStr, err4 := tools.Build(ctx)

	if err := firstErr(err1, err2, err3, err4); err != nil {
		return usersPorts{}, err
	}

	return usersPorts{agents: ags, account: accs, server: srvs, tools: tStr}, nil
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}
