package main

import (
	"os"

	"github.com/dees91/agent-skill-manager/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
