package cynosure

import (
	"context"
	"fmt"

	"github.com/goforj/wire"

	// "github.com/quenbyako/cynosure/internal/adapters/file"
	"github.com/quenbyako/cynosure/internal/adapters/gemini"
	"github.com/quenbyako/cynosure/internal/adapters/oauth"
	"github.com/quenbyako/cynosure/internal/adapters/sql"
	primitive "github.com/quenbyako/cynosure/internal/adapters/tool-handler"
	"github.com/quenbyako/cynosure/internal/adapters/zep"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

var (
	// fileAdapter = wire.NewSet(
	// 	newFileAdapter,
	// 	wire.Bind(new(ports.ModelSettingsStorageFactory), new(*file.File)),
	// 	wire.Bind(new(ports.AccountStorageFactory), new(*file.File)),
	// 	wire.Bind(new(ports.ServerStorageFactory), new(*file.File)),
	// )
	sqlAdapter = wire.NewSet(
		newSQLAdapter,
		wire.Bind(new(ports.ModelSettingsStorageFactory), new(*sql.Adapter)),
		wire.Bind(new(ports.AccountStorageFactory), new(*sql.Adapter)),
		wire.Bind(new(ports.ServerStorageFactory), new(*sql.Adapter)),
	)
	zepAdapter = wire.NewSet(
		newZepStorage,
		wire.Bind(new(ports.StorageRepositoryFactory), new(*zep.ZepStorage)),
	)
	geminiAdapter = wire.NewSet(newGeminiModel,
		wire.Bind(new(ports.ChatModelFactory), new(*gemini.GeminiModel)),
	)
	oauthAdapter = wire.NewSet(newOAuthHandler,
		wire.Bind(new(ports.OAuthHandlerFactory), new(*oauth.Handler)),
	)
	primitiveAdapter = wire.NewSet(primitive.NewHandler,
		wire.Bind(new(ports.ToolManagerFactory), new(*primitive.Handler)),
	)
)

// func newFileAdapter(p *appParams) *file.File {
// 	return file.New(p.storagePath)
// }

func newSQLAdapter(ctx context.Context, p *appParams) (*sql.Adapter, error) {
	if p.databaseURL == "" {
		return nil, fmt.Errorf("database URL is not configured")
	}

	return sql.NewAdapter(ctx, p.databaseURL)
}

func newZepStorage(ctx context.Context, p *appParams) *zep.ZepStorage {
	apiKey := must(p.zepKey.Get(ctx))

	return must(zep.NewZepStorage(
		zep.WithAPIKey(string(apiKey)),
	))
}

func newGeminiModel(ctx context.Context, p *appParams) (*gemini.GeminiModel, error) {
	geminiKey, err := p.geminiKey.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting gemini key from secret getter: %w", err)
	}

	return gemini.NewGeminiModel(
		ctx,
		&gemini.ClientConfig{
			APIKey: string(geminiKey),
		},
	)
}

func newOAuthHandler(p *appParams) *oauth.Handler {
	return oauth.New(
		p.oauthScopes,
		oauth.WithTracerProvider(p.observability),
	)
}
