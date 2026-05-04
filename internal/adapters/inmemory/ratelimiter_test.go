package inmemory_test

import (
	"context"
	"testing"

	"golang.org/x/time/rate"

	"github.com/quenbyako/cynosure/internal/adapters/inmemory"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter/testsuite"
)

func TestRateLimiter(t *testing.T) {
	t.Parallel()

	testsuite.Run(setupLimiter)(t)
}

func setupLimiter(_ context.Context, params testsuite.SetupParams) (ratelimiter.Port, error) {
	return inmemory.NewRateLimiter(
		rate.Every(params.Messages.Period),
		params.Messages.Limit,
		rate.Every(params.Tokens.Period),
		params.Tokens.Limit,
		params.MaxWait,
		params.Now,
		nil,
	), nil
}
