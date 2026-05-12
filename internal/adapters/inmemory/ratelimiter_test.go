package inmemory_test

import (
	"context"
	"testing"

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
		params.ChatInput.Period,
		params.ChatInput.Limit,
		params.ChatOutput.Period,
		params.ChatOutput.Limit,
		params.EmbeddingInput.Period,
		params.EmbeddingInput.Limit,
		params.MaxWait,
		params.Now,
		nil,
	), nil
}
