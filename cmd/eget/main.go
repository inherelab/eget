package main

import (
	"fmt"
	"os"

	"github.com/inherelab/eget/internal/cli"
)

// Build-time variables injected via -ldflags
var (
	Version   = "dev"
	GitHash   = "unknown"
	BuildTime = "unknown"
)

func main() {
	cli.SetBuildInfo(Version, GitHash, BuildTime)

	if err := cli.Main(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
