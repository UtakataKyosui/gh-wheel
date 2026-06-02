package main

import (
	"fmt"
	"os"

	"github.com/UtakataKyosui/gh-wheel/cmd"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version string

func main() {
	root := cmd.NewRootCmd()
	if version != "" {
		root.Version = version
	}
	// Error rendering and structured exit codes are wired in Issue #3
	// (internal/cliexit). Until then, simple stderr output.
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
