// Package accounts provides usecases for accounts management.
package accounts

import (
	"time"
)

const (
	pkgName = "github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/accounts"

	stateExpiration      = 5 * time.Minute
	discoveryPoolWorkers = 10
)
