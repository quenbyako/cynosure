package main

import (
	"os"

	"tg-helper/contrib/mongoose"

	"tg-helper/cmd/tghelper/root"
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
	os.Exit(goose.Run(root.Cmd)(ctx, os.Args[1:]))
}
