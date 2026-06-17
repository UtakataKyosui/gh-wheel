// Package auth provides gh binary presence, version, and token checks.
package auth

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	ghauth "github.com/cli/go-gh/v2/pkg/auth"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
)

// MinGHVersion is the minimum required gh CLI version.
const MinGHVersion = "2.40.0"

var versionRe = regexp.MustCompile(`gh version (\d+)\.(\d+)\.(\d+)`)

// CheckGH verifies that the gh binary is present in PATH.
func CheckGH() error {
	if _, err := exec.LookPath("gh"); err != nil {
		e := cliexit.NewAuth(cliexit.ErrCodeAuthNoBinary,
			fmt.Errorf("gh binary not found in PATH; install GitHub CLI from https://cli.github.com"))
		e.NextStep = "Install the GitHub CLI: https://cli.github.com"
		return e
	}
	return nil
}

// CheckGHVersion runs `gh --version` and returns an error if the version is
// older than MinGHVersion.
func CheckGHVersion() error {
	out, err := exec.Command("gh", "--version").Output()
	if err != nil {
		return cliexit.NewAuth(cliexit.ErrCodeAuthNoBinary,
			fmt.Errorf("failed to run gh --version: %w", err))
	}
	return ParseGHVersion(string(out))
}

// GHVersionString returns the parsed "X.Y.Z" gh CLI version, or "" if it
// cannot be determined. Best-effort and used only for diagnostics, so it never
// returns an error. A 2s timeout guards against `gh --version` hanging, which
// would otherwise block the crash-report path (including the panic handler).
func GHVersionString() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gh", "--version").Output()
	if err != nil {
		return ""
	}
	m := versionRe.FindStringSubmatch(string(out))
	if m == nil {
		return ""
	}
	return fmt.Sprintf("%s.%s.%s", m[1], m[2], m[3])
}

// ParseGHVersion is exported for unit testing without invoking gh.
// It parses the output of `gh --version` and returns an error if the version
// is older than 2.40.0.
func ParseGHVersion(output string) error {
	m := versionRe.FindStringSubmatch(output)
	if m == nil {
		return cliexit.NewAuth(cliexit.ErrCodeAuthOldGH,
			fmt.Errorf("could not parse gh version from output: %q", output))
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])

	// Derive the required minimum from MinGHVersion so both stay in sync.
	minParts := strings.SplitN(MinGHVersion, ".", 3)
	minMajor, _ := strconv.Atoi(minParts[0])
	minMinor, _ := strconv.Atoi(minParts[1])
	minPatch, _ := strconv.Atoi(minParts[2])

	tooOld := major < minMajor ||
		(major == minMajor && minor < minMinor) ||
		(major == minMajor && minor == minMinor && patch < minPatch)
	if tooOld {
		current := fmt.Sprintf("%d.%d.%d", major, minor, patch)
		e := cliexit.NewAuth(cliexit.ErrCodeAuthOldGH,
			fmt.Errorf("gh version %s is too old; %s or later is required", current, MinGHVersion))
		e.NextStep = "Run: gh upgrade"
		return e
	}
	return nil
}

// Token resolves an authentication token for the given host.
// If host is empty, "github.com" is used.
//
// Note: ghauth.TokenForHost returns (token, source) — the second value is
// the source string, NOT an error. An empty token means unauthenticated.
func Token(host string) (string, error) {
	if host == "" {
		host = "github.com"
	}
	token, _ := ghauth.TokenForHost(host) // second return is source, not error
	if token == "" {
		e := cliexit.NewAuth(cliexit.ErrCodeAuthNoToken,
			fmt.Errorf("not authenticated for %s; run `gh auth login` first", host))
		e.NextStep = "Run: gh auth login"
		return "", e
	}
	return token, nil
}
