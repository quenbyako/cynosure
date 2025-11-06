package ports

import (
	"context"
	"iter"

	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
)

type Agent interface {
	SendMessage(ctx context.Context, chat ids.MessageID, text components.MessageText) (iter.Seq2[components.MessageText, error], error)
}

type AgentFactory interface {
	Agent() Agent
}

func NewAgent(factory AgentFactory) Agent { return factory.Agent() }
