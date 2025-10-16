package ports

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
)

type Agent interface {
	SendMessage(ctx context.Context, chat ids.ChannelID, text components.MessageText) (chan components.MessageText, error)
}

type AgentFactory interface {
	Agent() Agent
}

func NewAgent(factory AgentFactory) Agent { return factory.Agent() }
