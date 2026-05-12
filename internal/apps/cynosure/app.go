package cynosure

import (
	"context"
)

type App struct {
	lifecycle *lifecycle
}

// Run starts all application background jobs and blocks
// until they finish or the context is canceled.
func (a *App) Run(ctx context.Context) error {
	return a.lifecycle.run(ctx)
}
