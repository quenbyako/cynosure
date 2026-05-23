// Package openrouter implements the OpenRouter chat model and embedding client
// adapters for Cynosure, supporting offline token counting and hybrid fallback.
package openrouter

import (
	"time"
)

const (
	pkgName = "github.com/quenbyako/cynosure/internal/adapters/openrouter"
)

const (
	defaultHardCap        = 50
	maxLimiterWait        = 2 * time.Second
	httpClientTimeout     = 2 * time.Minute
	sdkClientTimeout      = 30 * time.Second
	defaultMaxElapsedTime = 30 * time.Second
)
