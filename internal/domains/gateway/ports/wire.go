//go:build wireinject

package ports

import "github.com/goforj/wire"

var WirePorts = wire.NewSet(
	NewAgent,
	NewMessenger,
)
