package main

import (
	"os"

	"github.com/quenbyako/core"
	"github.com/quenbyako/core/contrib/runtime"

	"github.com/quenbyako/cynosure/cmd/cynosure/root"
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

	main := runtime.Run(root.Cmd)
	exitCode := main(ctx, os.Args[1:])

	os.Exit(int(exitCode))
}
