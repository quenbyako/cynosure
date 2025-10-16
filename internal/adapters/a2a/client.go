package a2a

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
	"github.com/quenbyako/cynosure/internal/domains/gateway/ports"
)

type Client struct {
}

var _ ports.AgentFactory = (*Client)(nil)

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Agent() ports.Agent { return c }

func (c *Client) SendMessage(ctx context.Context, chat ids.ChannelID, text components.MessageText) (chan components.MessageText, error) {
	return nil, nil
}
