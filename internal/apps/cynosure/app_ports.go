package cynosure

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/adapters/gemini"
	"github.com/quenbyako/cynosure/internal/adapters/openrouter"
	"github.com/quenbyako/cynosure/internal/adapters/sql"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/accounts"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/embedding"
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
	res, err := t.RateLimiter()
	if err != nil {
		return nil, fmt.Errorf("getting rate limiter: %w", err)
	}

	return res, nil
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
) constructor[accounts.PortWrapped] {
	return construct(func(ctx context.Context) (accounts.PortWrapped, error) {
		return selectPort(ctx, configuredAdapter(sqlImpl, false, accountStorage))
	})
}

func accountStorage[T accounts.PortFactory](t T) (accounts.PortWrapped, error) {
	return t.Accounts(), nil
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

func newChatModel(
	geminiImpl constructor[*gemini.GeminiModel],
	openrouterImpl constructor[*openrouter.Adapter],
) constructor[chatmodel.PortWrapped] {
	return construct(func(ctx context.Context) (chatmodel.PortWrapped, error) {
		return selectPort(ctx,
			configuredAdapter(geminiImpl, false, chatModel),
			configuredAdapter(openrouterImpl, false, chatModel),
		)
	})
}

func chatModel[T chatmodel.PortFactory](t T) (chatmodel.PortWrapped, error) {
	return t.ChatModel(), nil
}

func newEmbedding(
	geminiImpl constructor[*gemini.GeminiModel],
	openrouterImpl constructor[*openrouter.Adapter],
) constructor[embedding.PortWrapped] {
	return construct(func(ctx context.Context) (embedding.PortWrapped, error) {
		return selectPort(ctx,
			configuredAdapter(geminiImpl, false, embeddingModel),
			configuredAdapter(openrouterImpl, false, embeddingModel),
		)
	})
}

func embeddingModel[T embedding.PortFactory](t T) (embedding.PortWrapped, error) {
	return t.Embedding(), nil
}
