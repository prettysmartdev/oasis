// Package main is the entry point for the oasis CLI binary.
package main

import (
	"github.com/prettysmartdev/oasis/internal/cli"
)

// version is embedded at build time via -ldflags "-X main.version=$(git describe --tags --always)".
var version = "dev"

func main() {
	cli.Execute(version)
}
