//go:build wireinject

package ports

import "github.com/goforj/wire"

var WirePorts = wire.NewSet(
	NewAgentStorage,
	NewAccountStorage,
	NewServerStorage,
	NewThreadStorage,
	NewChatModel,
	NewOAuthHandler,
	NewToolClient,
	NewToolStorage,
	NewToolSemanticIndex,
	NewIdentityManager,
)
