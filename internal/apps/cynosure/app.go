// Package cynosure provides cynosure application.
package cynosure

import (
	"context"
	"errors"

	"github.com/quenbyako/core"
)

type App struct {
	telegramTaskRunner func(context.Context) error
	accountsTaskRunner func(context.Context) error
	ratelimiterCleanup func(context.Context) error
	tokenRefresherRun  func(context.Context) error
	mcpAdapterClose    func() error
}

// Run starts all application background jobs and blocks
// until they finish or the context is canceled.
func (a *App) Run(ctx context.Context) error {
	var errs []error
	if err := core.RunJobs(ctx,
		a.telegramTaskRunner,
		a.accountsTaskRunner,
		a.ratelimiterCleanup,
		a.tokenRefresherRun,
	); err != nil {
		errs = append(errs, err)
	}

	for _, closeFunc := range []func() error{
		a.mcpAdapterClose,
	} {
		if err := closeFunc(); err != nil {
			errs = append(errs, err)
		}
	}

	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return errors.Join(errs...)
	}
}
