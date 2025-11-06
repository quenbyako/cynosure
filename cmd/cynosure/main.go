package main

import (
	"context"
	"os"

	"github.com/quenbyako/core"
	"github.com/quenbyako/core/contrib/runtime"

	"github.com/quenbyako/cynosure/cmd/cynosure/root"
	"github.com/quenbyako/cynosure/cmd/cynosure/root/gateway"
)

//nolint:gochecknoglobals // ldflags doesn't work with constants
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, cancel := core.BuildContext(
		core.NewAppName("cynosure", "Cynosure"),
		core.NewVersion(version, commit, date),
		core.PipelineFromFiles(os.Stdin, os.Stdout, os.Stderr),
	)
	defer cancel()

	var cmd func(context.Context, []string) core.ExitCode
	if len(os.Args) == 1 {
		cmd = runtime.Run(root.Cmd)
	} else {
		switch os.Args[1] {
		case "gateway":
			cmd = runtime.Run(gateway.Cmd)
		default:
			panic("unknown subcommand" + os.Args[1])
		}
	}

	os.Exit(int(cmd(ctx, os.Args[1:])))
}
