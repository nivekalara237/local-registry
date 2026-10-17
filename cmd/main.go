package main

import (
	"os"

	"github.com/nivekalara237/local-registry/internal/cli"
)

// version is the release of this binary, set at build time:
//
//	go build -ldflags "-X main.version=v1.2.3" -o local-registry ./cmd
var version = "dev"

func main() {
	os.Exit(cli.Execute(version, os.Args[1:], os.Stdout, os.Stderr))
}
