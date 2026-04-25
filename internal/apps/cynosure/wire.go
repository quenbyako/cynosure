//go:build wireinject

package cynosure

import (
	"context"

	"github.com/goforj/wire"
	"github.com/quenbyako/core/contrib/runtime"

	"github.com/quenbyako/cynosure/internal/adapters/gemini"
	"github.com/quenbyako/cynosure/internal/adapters/inmemory"
	"github.com/quenbyako/cynosure/internal/adapters/mcp"
	"github.com/quenbyako/cynosure/internal/adapters/oauth"
	"github.com/quenbyako/cynosure/internal/adapters/ory"
	"github.com/quenbyako/cynosure/internal/adapters/sql"
	"github.com/quenbyako/cynosure/internal/controllers/telegram"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
	"github.com/quenbyako/cynosure/internal/logs"
)

var loggerConstructor = wire.NewSet(
	newLogger,
	wire.Bind(new(chat.LogCallbacks), new(*logs.BaseLogger)),
	wire.Bind(new(gemini.LogCallbacks), new(*logs.BaseLogger)),
	wire.Bind(new(telegram.LogCallbacks), new(*logs.BaseLogger)),
	wire.Bind(new(runtime.LogCallbacks), new(*logs.BaseLogger)),
)

var (
	sqlAdapter = wire.NewSet(newSQLAdapter,
		wire.Bind(new(ports.AgentStorageFactory), new(*sql.Adapter)),
		wire.Bind(new(ports.AccountStorageFactory), new(*sql.Adapter)),
		wire.Bind(new(ports.ServerStorageFactory), new(*sql.Adapter)),
		wire.Bind(new(ports.ThreadStorageFactory), new(*sql.Adapter)),
		wire.Bind(new(ports.ToolStorageFactory), new(*sql.Adapter)),
	)
	geminiAdapter = wire.NewSet(newGeminiModel,
		wire.Bind(new(chatmodel.PortFactory), new(*gemini.GeminiModel)),
		wire.Bind(new(ports.ToolSemanticIndexFactory), new(*gemini.GeminiModel)),
	)
	oauthAdapter = wire.NewSet(newOAuthHandler,
		wire.Bind(new(oauthhandler.Factory), new(*oauth.Handler)),
	)
	mcpAdapter = wire.NewSet(newMCPHandler,
		wire.Bind(new(toolclient.PortFactory), new(*mcp.Handler)),
	)
	oryAdapter = wire.NewSet(newOryClient,
		wire.Bind(new(identitymanager.PortFactory), new(*ory.Adapter)),
	)
	ratelimiterAdapter = wire.NewSet(newRateLimiter,
		wire.Bind(new(ratelimiter.PortFactory), new(*inmemory.RateLimiter)),
	)
)

var (
	chatUsecase     = wire.NewSet(newChatUsecase)
	accountsUsecase = wire.NewSet(newAccountsUsecase)
	usersUsecase    = wire.NewSet(newUsersUsecase)
)

var controllersSet = wire.NewSet(
	bindAdminController,
	bindOAuthController,
	bindTelegramController,
	bindMCPController,
)

func buildApp(ctx context.Context, config *appParams) (*App, error) {
	panic(wire.Build(
		ports.WirePorts,
		chatmodel.New,
		identitymanager.New,
		oauthhandler.New,
		ratelimiter.New,
		toolclient.New,

		loggerConstructor,

		sqlAdapter,
		geminiAdapter,
		mcpAdapter,
		oauthAdapter,
		oryAdapter,
		ratelimiterAdapter,

		chatUsecase,
		accountsUsecase,
		usersUsecase,

		controllersSet,

		connectDependencies,
	))
}
