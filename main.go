package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/UtakataKyosui/gh-wheel/cmd"
	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
// Falls back to "dev" for local/untagged builds so --version is always registered.
var version string

func main() {
	if version == "" {
		version = "dev"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := cmd.NewRootCmd()
	root.Version = version

	if err := root.ExecuteContext(ctx); err != nil {
		wrapped := cobraUsageErr(err)
		asJSON, _ := root.PersistentFlags().GetBool("json")
		cliexit.Render(wrapped, asJSON, os.Stdout, os.Stderr)
		os.Exit(cliexit.ExitCodeOf(wrapped))
	}
}

// cobraUsageErr remaps cobra's built-in usage errors (unknown command, unknown
// flag, missing required flag, wrong argument count) to a *cliexit.Error with
// exit code 2 (CodeUsage). Non-usage errors become CodeGeneral (exit 1).
func cobraUsageErr(err error) error {
	if err == nil {
		return nil
	}
	var ce *cliexit.Error
	if errors.As(err, &ce) {
		return err // already structured — leave alone
	}
	msg := err.Error()
	if strings.HasPrefix(msg, "unknown command") ||
		strings.HasPrefix(msg, "unknown flag") ||
		strings.HasPrefix(msg, "unknown shorthand flag") ||
		strings.HasPrefix(msg, "accepts") ||
		strings.HasPrefix(msg, "required flag") ||
		strings.HasPrefix(msg, "invalid argument") ||
		strings.HasPrefix(msg, "flag needs an argument") {
		return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs, err)
	}
	return cliexit.NewGeneral(err)
}
