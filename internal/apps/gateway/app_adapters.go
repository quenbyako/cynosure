package gateway

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/wire"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/quenbyako/cynosure/internal/adapters/a2a"
	"github.com/quenbyako/cynosure/internal/adapters/telegram"
	"github.com/quenbyako/cynosure/internal/domains/gateway/ports"
)

var (
	telegramAdapter = wire.NewSet(
		newTelegramAdapter,
		wire.Bind(new(ports.MessengerFactory), new(*telegram.Messenger)),
	)

	a2aAdapter = wire.NewSet(
		newA2AAdapter,
		wire.Bind(new(ports.AgentFactory), new(*a2a.Client)),
	)
)

func newTelegramAdapter(ctx context.Context, p *appParams) (*telegram.Messenger, error) {
	token, err := p.telegramToken.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting token for telegram: %w", err)
	}

	return telegram.NewMessenger(string(token), telegram.WithWebhook(tgbotapi.WebhookConfig{
		URL: p.webhookAddr,
	}))
}

func newA2AAdapter(ctx context.Context, p *appParams) (*a2a.Client, error) {
	grpcClient, err := grpc.NewClient(p.a2aClient.Host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("creating grpc client for a2a: %w", err)
	}

	return a2a.NewClient(grpcClient), nil
}
