package main

import (
	"fmt"
	"os"

	"github.com/inherelab/eget/internal/cli"
)

func main() {
	if err := cli.Main(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
