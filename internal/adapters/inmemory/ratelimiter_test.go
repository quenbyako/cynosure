package inmemory_test

import (
	"context"
	"testing"
	"time"

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
		rate.Limit(params.Limit),
		params.Burst,
		time.Hour, // large TTL for tests
		params.Now,
		nil,
	), nil
}
