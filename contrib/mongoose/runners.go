package goose

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"reflect"

	"github.com/caarlos0/env/v11"
)

type AppCtx[T any] struct {
	Stdin      io.Reader
	Stdout     io.Writer
	Log        slog.Handler
	Flags      T
	Version    Version
	IsPipeline bool
}

type ActionFunc[T any] func(ctx context.Context, appCtx AppCtx[T]) int

type FlagDef interface {
	CustomMappers() map[reflect.Type]env.ParserFunc

	// required environments
	GetLogLevel() slog.Level
}

// Run is a helper function for cobra.Command.PreRun that loads flags
// from environment and merges them with flags from command line.
func Run[T FlagDef](action ActionFunc[T]) func(context.Context, []string) int {
	return func(ctx context.Context, _ []string) int {
		var flags T

		mappers := map[reflect.Type]env.ParserFunc{
			reflect.TypeFor[slog.Level](): func(v string) (any, error) { return parseLogLevel(v) },
		}
		for k, v := range flags.CustomMappers() {
			mappers[k] = v
		}

		if err := env.ParseWithOptions(&flags, env.Options{
			TagName:             "env",
			PrefixTagName:       "prefix",
			DefaultValueTagName: "default",
			RequiredIfNoDef:     true,
			Environment:         env.ToMap(os.Environ()),
			FuncMap:             mappers,
		}); err != nil {
			panic(fmt.Errorf("parsing flags from environment: %w", err))
		}

		log := defaultLogger(os.Stderr, flags.GetLogLevel())

		version, _ := VersionFromContext(ctx)
		pipes, _ := pipelineFromContext(ctx)
		return action(ctx, AppCtx[T]{
			IsPipeline: pipes.isPipeline,
			Stdin:      os.Stdin,
			Stdout:     os.Stdout,
			Log:        log,
			Flags:      flags,
			Version:    version,
		})
	}
}
