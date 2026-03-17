// Package root defines root command for cynosure application.
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
		cynosure.WithHTTPServer(cfg.HTTPPort.Register),
		cynosure.WithGeminiKey(cfg.GeminiKey),
		cynosure.WithTelegramKey(cfg.TelegramKey),
		cynosure.WithTelegramServer(cfg.TelegramPort.Register),
		cynosure.WithTelegramPublicAddr(cfg.TelegramPublicAddr),
		cynosure.WithOry(cfg.OryEndpoint, cfg.OryAdminKey),
		cynosure.WithOryClientCredentials(cfg.OryClientID, cfg.OryClientSecret),
		cynosure.WithOAuthCallbackURL(cfg.OAuthRedirectURL),
		cynosure.WithMCP(cfg.MCPPort.Register),
		cynosure.WithAdminMCPID(cfg.AdminMCPServerID),
	}
	if cfg.DatabaseURL != nil && cfg.DatabaseURL.Scheme != "" {
		opts = append(opts, cynosure.WithDatabaseURL(cfg.DatabaseURL))
	}

	metrics, ok := core.Observability(appCtx)
	if ok {
		opts = append(opts, cynosure.WithObservability(metrics))
	}

	_, err := cynosure.Build(ctx, opts...)
	if err != nil {
		panic(err) //nolint:forbidigo // safe to use here.
	}

	return runJobs(ctx, &cfg)
}

func runJobs(ctx context.Context, cfg *Config) core.ExitCode {
	jobs := []func(context.Context) error{
		cfg.Port.Serve,
		cfg.HTTPPort.Serve,
		cfg.TelegramPort.Serve,
		cfg.MCPPort.Serve, // TODO: force disable for mcp servers? How to do that?
	}

	if err := core.RunJobs(ctx, jobs...); err != nil {
		//nolint:forbidigo // it's WAY easier to log like that. we don't expect any issues here
		fmt.Println("Oopsie: ", err)

		return 1
	}

	return 0
}
