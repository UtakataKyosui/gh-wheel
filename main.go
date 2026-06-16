package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/UtakataKyosui/gh-wheel/cmd"
	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/issuereport"
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

	// Recover panics so the user is offered a pre-filled, masked crash report
	// instead of a bare Go stack trace. os.Exit in the normal paths below skips
	// deferred funcs, so this only fires on an actual panic.
	defer func() {
		if r := recover(); r != nil {
			// Always print the panic and its stack to stderr first, so the
			// original Go trace survives even when the crash reporter is
			// suppressed (non-interactive/--json/opt-out) — preserving
			// CI/log-based debugging.
			stack := debug.Stack()
			fmt.Fprintf(os.Stderr, "panic: %v\n\n%s\n", r, stack)
			asJSON, interactive, optOut := reportFlags(root)
			path, rest := commandContext(root, os.Args)
			issuereport.OfferPanic(path, rest, version, r, stack,
				asJSON, interactive, optOut, os.Stdin, os.Stderr)
			os.Exit(cliexit.CodeGeneral)
		}
	}()

	if err := root.ExecuteContext(ctx); err != nil {
		wrapped := cobraUsageErr(err)
		asJSON, interactive, optOut := reportFlags(root)
		cliexit.Render(wrapped, asJSON, os.Stdout, os.Stderr)

		// On an unexpected (internal) error, offer to file an issue. No-op for
		// auth/usage/validation/API errors and in non-interactive/--json/opt-out.
		path, rest := commandContext(root, os.Args)
		issuereport.Offer(path, rest, version, wrapped,
			asJSON, interactive, optOut, os.Stdin, os.Stderr)

		os.Exit(cliexit.ExitCodeOf(wrapped))
	}
}

// reportFlags resolves the asJSON, interactive, and optOut conditions shared by
// issuereport.Offer and issuereport.OfferPanic.
func reportFlags(root *cobra.Command) (asJSON, interactive, optOut bool) {
	asJSON, _ = root.PersistentFlags().GetBool("json")
	noReport, _ := root.PersistentFlags().GetBool("no-report")
	interactive = term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stderr.Fd())
	optOut = optedOut(noReport, os.Getenv(issuereport.OptOutEnv))
	return asJSON, interactive, optOut
}

// optedOut reports whether issue reporting is disabled, via either the
// --no-report flag or the GH_WHEEL_NO_REPORT environment variable.
func optedOut(noReport bool, env string) bool {
	return noReport || env != ""
}

// commandContext resolves the path of the command cobra would dispatch to and
// the remaining args for it, so the report records the leaf command (e.g.
// "wheel review post") and only that command's flags get redacted. argv is the
// full process argument vector (os.Args); the empty-argv guard prevents a
// nested panic when commandContext runs from inside the recover handler.
func commandContext(root *cobra.Command, argv []string) (string, []string) {
	if len(argv) == 0 {
		return root.CommandPath(), nil
	}
	target, rest, err := root.Find(argv[1:])
	if err != nil || target == nil {
		return root.CommandPath(), argv[1:]
	}
	return target.CommandPath(), rest
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
