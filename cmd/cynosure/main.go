package main

import (
	"context"
	"os"

	"github.com/quenbyako/cynosure/contrib/mongoose"

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
	ctx, cancel := goose.BuildContext(
		os.Stdin,
		os.Stdout,
		os.Stderr,
		goose.Version{Version: version, Commit: commit, Date: date},
	)
	defer cancel()

	// cobra errors are incredibly useless: ExecuteContext prints help message
	// and returns string error (which is already printed), so it's completely
	// useless to check error at all here.

	var cmd func(context.Context, []string) int
	if len(os.Args) == 1 {
		cmd = goose.Run(root.Cmd)
	} else {
		switch os.Args[1] {
		case "gateway":
			cmd = goose.Run(gateway.Cmd)
		default:
			panic("unknown subcommand" + os.Args[1])
		}
	}

	os.Exit(cmd(ctx, os.Args[1:]))

}
