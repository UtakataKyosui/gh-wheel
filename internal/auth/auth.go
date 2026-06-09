// Package auth provides gh binary presence, version, and token checks.
package auth

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	ghauth "github.com/cli/go-gh/v2/pkg/auth"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
)

// MinGHVersion is the minimum required gh CLI version.
const MinGHVersion = "2.40.0"

var versionRe = regexp.MustCompile(`gh version (\d+)\.(\d+)\.(\d+)`)

// CheckGH verifies that the gh binary is present in PATH.
func CheckGH() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return cliexit.NewAuth(cliexit.ErrCodeAuthNoBinary,
			fmt.Errorf("gh binary not found in PATH; install GitHub CLI from https://cli.github.com"))
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
		return cliexit.NewAuth(cliexit.ErrCodeAuthOldGH,
			fmt.Errorf("gh version %s is too old; %s or later is required", current, MinGHVersion))
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
		return "", cliexit.NewAuth(cliexit.ErrCodeAuthNoToken,
			fmt.Errorf("not authenticated for %s; run `gh auth login` first", host))
	}
	return token, nil
}
