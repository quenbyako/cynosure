package goose

import (
	"context"
	"os"
	"os/signal"
)

// BuildContext creates a new context with all wrappers, including:
//   - context attached to signals
//   - logger injected in context
//   - version injected in context
func BuildContext(
	stdin, stdout, stderr *os.File,
	version Version,
) (
	ctx context.Context,
	cancel context.CancelFunc,
) {
	ctx, cancel = signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	ctx = WithVersion(ctx, version)
	ctx = withPipelines(ctx, pipelineFromFiles(stdin, stdout, stderr))

	return ctx, cancel
}
