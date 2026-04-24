// Package root defines root command for cynosure application.
package root

import (
	"context"
	"errors"
	"fmt"

	"github.com/quenbyako/core"

	"github.com/quenbyako/cynosure/internal/apps/cynosure"
)

var _ core.ActionFunc[Config] = Cmd

// Cmd is the entry point for the cynosure application command.
//
//nolint:funlen // it's a bridge between config and app, mapping is naturally long.
func Cmd(ctx context.Context, appCtx core.AppContext[Config]) core.ExitCode {
	cfg := appCtx.Config()

	opts := []cynosure.AppOpts{
		cynosure.WithGRPCServer(cfg.Port),
		cynosure.WithHTTPServer(cfg.HTTPPort.Register),
		cynosure.WithGeminiKey(cfg.GeminiKey),
		cynosure.WithGeminiClient(cfg.GeminiClient),
		cynosure.WithTelegramKey(cfg.TelegramKey),
		cynosure.WithTelegramServer(cfg.TelegramPort.Register),
		cynosure.WithTelegramPublicAddr(cfg.TelegramPublicAddr),
		cynosure.WithTelegramClient(cfg.TelegramClient),
		cynosure.WithOry(cfg.OryEndpoint, cfg.OryAdminKey),
		cynosure.WithOryClientCredentials(cfg.OryClientID, cfg.OryClientSecret),
		cynosure.WithOAuthCallbackURL(cfg.OAuthRedirectURL),
		cynosure.WithMCP(cfg.MCPPort.Register),
		cynosure.WithAdminMCPID(cfg.AdminMCPServerID),
		cynosure.WithRateLimit(cfg.RateLimit),
		cynosure.WithChatLimits(cfg.ChatSoftLimit, cfg.ChatHardCap),
	}

	if cfg.DatabaseURL != nil && cfg.DatabaseURL.Scheme != "" {
		opts = append(opts, cynosure.WithDatabaseURL(cfg.DatabaseURL))
	}

	if metrics, ok := core.Observability(appCtx); ok {
		opts = append(opts, cynosure.WithObservability(metrics))
	}

	app, err := cynosure.Build(ctx, opts...)
	if err != nil {
		panic(err) //nolint:forbidigo // safe to use here.
	}

	return runJobs(ctx, &cfg, app)
}

func runJobs(
	ctx context.Context,
	cfg *Config,
	app *cynosure.App,
) core.ExitCode {
	jobs := []func(context.Context) error{
		cfg.Port.Serve,
		cfg.HTTPPort.Serve,
		cfg.TelegramPort.Serve,
		cfg.MCPPort.Serve, // TODO: force disable for mcp servers? How to do that?
		app.Run,
	}

	if err := core.RunJobs(ctx, jobs...); err != nil {
		if errors.Is(err, context.Canceled) {
			return 0
		}

		//nolint:forbidigo // logging to stdout is acceptable in the root command.
		fmt.Println("Oopsie: ", err)

		return 1
	}

	return 0
}
