package main

import (
	"fmt"
	"os"

	"github.com/inherelab/eget/internal/cli"
	"github.com/inherelab/eget/internal/version"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("eget version", version.Version)
		return
	}
	if err := cli.Main(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
