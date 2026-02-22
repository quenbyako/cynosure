package root

import (
	"context"
	"fmt"

	"github.com/quenbyako/core"

	"github.com/quenbyako/cynosure/internal/apps/cynosure"
)

var _ core.ActionFunc[Config] = Cmd

func Cmd(ctx context.Context, appCtx core.AppContext[Config]) core.ExitCode {
	cfg := appCtx.Config()

	opts := []cynosure.AppOpts{
		cynosure.WithGRPCServer(cfg.Port),
		cynosure.WithHTTPServer(cfg.HttpPort.Register),
		cynosure.WithGeminiKey(cfg.GeminiKey),
		cynosure.WithTelegramKey(cfg.TelegramKey),
		cynosure.WithTelegramServer(cfg.TelegramPort.Register),
		cynosure.WithTelegramPublicAddr(cfg.TelegramPublicAddr),
		cynosure.WithDefaultModelConfig("e0689c78-4fd0-4eca-a907-8e00515bc88d"),
		cynosure.WithOry(cfg.OryEndpoint, cfg.OryAdminKey),
		cynosure.WithMCP(cfg.MCPPort.Register),
	}

	if cfg.DatabaseURL != nil || cfg.DatabaseURL.Scheme != "" {
		opts = append(opts, cynosure.WithDatabaseURL(cfg.DatabaseURL))
	}

	if metrics, ok := core.Observability(appCtx); ok {
		opts = append(opts, cynosure.WithObservability(metrics))
	}

	_ = cynosure.Build(ctx, opts...)

	jobs := []func(context.Context) error{
		cfg.Port.Serve,
		cfg.HttpPort.Serve,
		cfg.TelegramPort.Serve,
		cfg.MCPPort.Serve, // TODO: force disable for mcp servers? How to do that?
	}

	if err := core.RunJobs(ctx, jobs...); err != nil {
		fmt.Println("Oopsie: ", err)

		return 1
	}

	return 0
}
