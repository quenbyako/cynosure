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
		cynosure.WithZepKey(cfg.ZepKey),
		cynosure.WithGeminiKey(cfg.GeminiKey),
		cynosure.WithDefaultModelConfig("e0689c78-4fd0-4eca-a907-8e00515bc88d"),
	}

	if metrics, ok := core.Observability(appCtx); ok {
		opts = append(opts, cynosure.WithObservability(metrics))
	}

	_ = cynosure.NewApp(ctx, opts...)

	jobs := []func(context.Context) error{
		cfg.Port.Serve,
		cfg.HttpPort.Serve,
	}

	if err := core.RunJobs(ctx, jobs...); err != nil {
		fmt.Println("Oopsie: ", err)

		return 1
	}

	return 0
}
