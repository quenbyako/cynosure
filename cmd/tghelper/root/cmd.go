package root

import (
	"context"
	"log/slog"
	"reflect"

	"github.com/caarlos0/env/v11"
	goose "tg-helper/contrib/mongoose"

	"tg-helper/internal/apps/tghelper"
)

type Flags struct {
	LogLevel slog.Level `env:"LOG_LEVEL" default:"info"`
	Port     uint16     `env:"GRPC_PORT" default:"5001"`
	HttpPort uint16     `env:"HTTP_PORT" default:"5002"`
}

func (f Flags) CustomMappers() map[reflect.Type]env.ParserFunc {
	return nil
}

func (f Flags) GetLogLevel() slog.Level { return f.LogLevel }

var _ goose.FlagDef = (*Flags)(nil)

func Cmd(ctx context.Context, appCtx goose.AppCtx[Flags]) int {
	app := tghelper.NewApp(ctx,
		tghelper.WithLog(appCtx.Log),
		tghelper.WithGRPCPort(appCtx.Flags.Port),
		tghelper.WithHTTPPort(appCtx.Flags.HttpPort),
		tghelper.WithZepKey("<REDACTED>"),
		tghelper.WithTraceEndpoint("localhost:4317"),
		tghelper.WithDefaultModelConfig("e0689c78-4fd0-4eca-a907-8e00515bc88d"),
	)

	goose.RunJobs(ctx,
		app.StartGRPC,
		app.StartOAuth,
	)

	return 0
}
