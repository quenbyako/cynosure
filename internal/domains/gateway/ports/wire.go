//go:build wireinject

package ports

import "github.com/google/wire"

var WirePorts = wire.NewSet(
	NewAgent,
	NewMessenger,
)
