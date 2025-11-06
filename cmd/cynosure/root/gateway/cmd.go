package gateway

import (
	"context"
	"fmt"
	"net/url"

	"github.com/quenbyako/core"

	"github.com/quenbyako/cynosure/internal/apps/gateway"
)

var _ core.ActionFunc[Config] = Cmd

func Cmd(ctx context.Context, appCtx core.AppContext[Config]) core.ExitCode {
	cfg := appCtx.Config()
	u, _ := url.Parse("grpc://localhost:5001")

	opts := []gateway.AppOpts{
		gateway.WithTelegramToken(cfg.TelegramToken),
		gateway.WithWebhookPort(cfg.HttpPort.Register),
		gateway.WithWebhookAddress(cfg.WebhookAddr),
		gateway.WithA2AClientAddress(*u),
	}

	if metrics, ok := core.Observability(appCtx); ok {
		opts = append(opts, gateway.WithObservability(metrics))
	}

	_ = gateway.NewApp(ctx, opts...)

	jobs := []func(context.Context) error{
		cfg.HttpPort.Serve,
	}

	if err := core.RunJobs(ctx, jobs...); err != nil {
		fmt.Println("Oopsie: ", err)

		return 1
	}

	return 0
}
