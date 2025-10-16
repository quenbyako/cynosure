//go:build wireinject

package ports

import "github.com/google/wire"

var WirePorts = wire.NewSet(
	NewModelSettingsStorage,
	NewAccountStorage,
	NewServerStorage,
	NewStorageRepository,
	NewToolManager,
	NewChatModel,
	NewOAuthHandler,
	NewToolCache,
)
