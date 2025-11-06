package cynosure

import (
	"context"
	"fmt"

	"github.com/google/wire"

	"github.com/quenbyako/cynosure/internal/adapters/file"
	"github.com/quenbyako/cynosure/internal/adapters/gemini"
	oauthClient "github.com/quenbyako/cynosure/internal/adapters/oauth"
	primitive "github.com/quenbyako/cynosure/internal/adapters/tool-handler"
	"github.com/quenbyako/cynosure/internal/adapters/zep"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

var (
	fileAdapter = wire.NewSet(
		newFileAdapter,
		wire.Bind(new(ports.ModelSettingsStorageFactory), new(*file.File)),
		wire.Bind(new(ports.AccountStorageFactory), new(*file.File)),
		wire.Bind(new(ports.ServerStorageFactory), new(*file.File)),
	)
	zepAdapter = wire.NewSet(
		newZepStorage,
		wire.Bind(new(ports.StorageRepositoryFactory), new(*zep.ZepStorage)),
	)
	geminiAdapter = wire.NewSet(newGeminiModel,
		wire.Bind(new(ports.ChatModelFactory), new(*gemini.GeminiModel)),
	)
	oauthAdapter = wire.NewSet(newOAuthHandler,
		wire.Bind(new(ports.OAuthHandlerFactory), new(*oauthClient.Handler)),
	)
	primitiveAdapter = wire.NewSet(primitive.NewHandler,
		wire.Bind(new(ports.ToolManagerFactory), new(*primitive.Handler)),
	)
)

func newFileAdapter(p *appParams) *file.File {
	return file.New(p.storagePath)
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

func newOAuthHandler(p *appParams) *oauthClient.Handler {
	return oauthClient.New(
		p.oauthScopes,
		oauthClient.WithTracerProvider(p.observability),
	)
}
