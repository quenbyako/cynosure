//go:build wireinject

package ports

import "github.com/goforj/wire"

var WirePorts = wire.NewSet(
	NewModelSettingsStorage,
	NewAccountStorage,
	NewServerStorage,
	NewStorageRepository,
	NewToolManager,
	NewChatModel,
	NewOAuthHandler,
)
