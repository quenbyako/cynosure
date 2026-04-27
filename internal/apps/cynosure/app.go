// Package cynosure provides cynosure application.
package cynosure

import (
	"context"

	"github.com/quenbyako/core"
)

type App struct {
	telegramTaskRunner func(context.Context) error
	ratelimiterCleanup func(context.Context) error
}

// Run starts all application background jobs and blocks
// until they finish or the context is canceled.
func (a *App) Run(ctx context.Context) error {
	//nolint:wrapcheck // propagating job runner error natively
	return core.RunJobs(ctx,
		a.telegramTaskRunner,
		a.ratelimiterCleanup,
	)
}
