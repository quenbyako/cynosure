//go:build wireinject

package cynosure

import (
	"context"

	"github.com/goforj/wire"
	"github.com/quenbyako/core/contrib/runtime"

	"github.com/quenbyako/cynosure/internal/adapters/gemini"
	"github.com/quenbyako/cynosure/internal/adapters/mcp"
	"github.com/quenbyako/cynosure/internal/adapters/oauth"
	"github.com/quenbyako/cynosure/internal/adapters/ory"
	"github.com/quenbyako/cynosure/internal/controllers/telegram"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/embedding"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
	"github.com/quenbyako/cynosure/internal/logs"
)

func buildApp(context.Context, *appParams) (*App, error) {
	panic(wire.Build(
		chatmodel.New,
		embedding.New,
		identitymanager.New,
		oauthhandler.New,
		toolclient.New,

		wire.NewSet(
			newLogger,
			wire.Bind(new(gemini.LogCallbacks), new(*logs.BaseLogger)),
			wire.Bind(new(telegram.LogCallbacks), new(*logs.BaseLogger)),
			wire.Bind(new(runtime.LogCallbacks), new(*logs.BaseLogger)),
		),
		newLifecycle,

		wire.NewSet(newGeminiModel,
			wire.Bind(new(chatmodel.PortFactory), new(*gemini.GeminiModel)),
			wire.Bind(new(embedding.PortFactory), new(*gemini.GeminiModel)),
		),
		wire.NewSet(newMCPHandler,
			wire.Bind(new(toolclient.PortFactory), new(*mcp.Handler)),
		),
		wire.NewSet(newOauthRefresher),
		wire.NewSet(newOAuthHandler,
			wire.Bind(new(oauthhandler.Factory), new(*oauth.Handler)),
		),
		wire.NewSet(newOryClient,
			wire.Bind(new(identitymanager.PortFactory), new(*ory.Adapter)),
		),
		newPostgres,

		newRateLimiter,
		newAgentStorage,
		newAccountStorage,
		newServerStorage,
		newThreadStorage,
		newToolStorage,

		newChatUsecase,
		newAccountsUsecase,
		newUsersUsecase,

		bindAdminController,
		bindOAuthController,
		bindTelegramController,
		bindMCPController,

		connectDependencies,
	))
}
