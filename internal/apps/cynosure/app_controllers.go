package cynosure

import (
	"context"
	"fmt"

	"go.opentelemetry.io/contrib/bridges/otelslog"

	"github.com/quenbyako/cynosure/internal/controllers/admin"
	"github.com/quenbyako/cynosure/internal/controllers/mcp"
	"github.com/quenbyako/cynosure/internal/controllers/oauth"
	"github.com/quenbyako/cynosure/internal/controllers/telegram"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/accounts"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/users"
	"github.com/quenbyako/cynosure/internal/logs"
)

type (
	adminControllerWireBind    struct{}
	oauthControllerWireBind    struct{}
	telegramControllerWireBind struct{}
	mcpControllerWireBind      struct{}
)

func bindAdminController(
	params *appParams,
	usecase *accounts.Usecase,
) adminControllerWireBind { //nolint:unparam // wire bind
	admin.Register(usecase)(params.grpcAddr)

	return adminControllerWireBind{}
}

func bindOAuthController(
	params *appParams,
	usecase *accounts.Usecase,
) oauthControllerWireBind { //nolint:unparam // wire bind
	params.httpAddr(oauth.NewHandler(usecase))

	return oauthControllerWireBind{}
}

func bindTelegramController(
	ctx context.Context,
	params *appParams,
	lifecycle *lifecycle,
	log *logs.BaseLogger,
	chatUsecase *chat.Usecase,
	usersUsecase *users.Usecase,
) (telegramControllerWireBind, error) { //nolint:unparam // wire bind
	telegramKey, err := params.telegram.key.Get(ctx)
	if err != nil {
		return telegramControllerWireBind{}, fmt.Errorf("getting telegram key: %w", err)
	}

	handler, job, err := telegram.New(
		ctx,
		chatUsecase,
		usersUsecase,
		params.telegram.publicAddr,
		telegramKey,
		telegram.WithLogCallbacks(log),
		telegram.WithTracer(params.observability),
		telegram.WithClient(params.telegram.apiClient),
	)
	if err != nil {
		return telegramControllerWireBind{}, fmt.Errorf("creating telegram controller: %w", err)
	}

	params.telegram.register(handler)
	lifecycle.schedule(job)

	return telegramControllerWireBind{}, nil
}

func bindMCPController(
	params *appParams,
	usecase *accounts.Usecase,
) (mcpControllerWireBind, error) { //nolint:unparam // wire bind
	handler, err := mcp.New(
		usecase,
		mcpImpl,
		mcp.WithLogger(otelslog.NewHandler("mcp",
			otelslog.WithLoggerProvider(params.observability),
		)),
		mcp.WithAllowedIssuers(params.ory.endpoint.Host),
		mcp.WithHTTPClient(params.ory.apiClient),
	)
	if err != nil {
		return mcpControllerWireBind{}, fmt.Errorf("creating mcp controller: %w", err)
	}

	params.mcpAddr(handler)

	return mcpControllerWireBind{}, nil
}
