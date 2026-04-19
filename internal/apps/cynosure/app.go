// Package cynosure provides cynosure application.
package cynosure

import (
	"context"

	"github.com/quenbyako/core"
)

type App struct {
	jobs []func(context.Context) error
}

// Run starts all application background jobs and blocks
// until they finish or the context is canceled.
func (a *App) Run(ctx context.Context) error {
	if len(a.jobs) == 0 {
		<-ctx.Done()
		return ctx.Err() //nolint:wrapcheck // propagating context cancellation
	}

	//nolint:wrapcheck // propagating job runner error natively
	return core.RunJobs(ctx, a.jobs...)
}
