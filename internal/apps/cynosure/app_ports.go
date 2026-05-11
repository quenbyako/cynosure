package cynosure

import (
	"context"

	"github.com/quenbyako/cynosure/internal/adapters/sql"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
)

func newRateLimiter(
	// inmemImpl constructor[*inmemory.RateLimiter],
	sqlImpl constructor[*sql.Adapter],
) constructor[ratelimiter.PortWrapped] {
	return construct(func(ctx context.Context) (ratelimiter.PortWrapped, error) {
		return selectPort(ctx,
			// fallbackAdapter(inmemImpl, false, rateLimiter),
			configuredAdapter(sqlImpl, false, rateLimiter),
		)
	})
}

func rateLimiter[T ratelimiter.PortFactory](t T) (ratelimiter.PortWrapped, error) {
	return t.RateLimiter()
}

func newAgentStorage(
	sqlImpl constructor[*sql.Adapter],
) constructor[ports.AgentStorage] {
	return construct(func(ctx context.Context) (ports.AgentStorage, error) {
		return selectPort(ctx,
			configuredAdapter(sqlImpl, false, agentStorage),
		)
	})
}

func agentStorage[T ports.AgentStorageFactory](t T) (ports.AgentStorage, error) {
	return t.AgentStorage(), nil
}

func newAccountStorage(
	sqlImpl constructor[*sql.Adapter],
) constructor[ports.AccountStorage] {
	return construct(func(ctx context.Context) (ports.AccountStorage, error) {
		return selectPort(ctx, configuredAdapter(sqlImpl, false, accountStorage))
	})
}

func accountStorage[T ports.AccountStorageFactory](t T) (ports.AccountStorage, error) {
	return t.AccountStorage(), nil
}

func newServerStorage(
	sqlImpl constructor[*sql.Adapter],
) constructor[ports.ServerStorage] {
	return construct(func(ctx context.Context) (ports.ServerStorage, error) {
		return selectPort(ctx,
			configuredAdapter(sqlImpl, false, serverStorage),
		)
	})
}

func serverStorage[T ports.ServerStorageFactory](t T) (ports.ServerStorage, error) {
	return t.ServerStorage(), nil
}

func newThreadStorage(
	sqlImpl constructor[*sql.Adapter],
) constructor[ports.ThreadStorageWrapped] {
	return construct(func(ctx context.Context) (ports.ThreadStorageWrapped, error) {
		return selectPort(ctx,
			configuredAdapter(sqlImpl, false, threadStorage),
		)
	})
}

func threadStorage[T ports.ThreadStorageFactory](t T) (ports.ThreadStorageWrapped, error) {
	return t.ThreadStorage(), nil
}

func newToolStorage(
	sqlImpl constructor[*sql.Adapter],
) constructor[ports.ToolStorage] {
	return construct(func(ctx context.Context) (ports.ToolStorage, error) {
		return selectPort(ctx,
			configuredAdapter(sqlImpl, false, toolStorage),
		)
	})
}

func toolStorage[T ports.ToolStorageFactory](t T) (ports.ToolStorage, error) {
	return t.ToolStorage(), nil
}
