// Package notify shows desktop notifications to the user.
//
// On macOS it shells out to osascript; on every other platform Notify is a
// no-op so callers (such as the schedule daemon) stay cross-platform without
// build tags. Use Available to detect whether a notification would actually be
// shown before promising the user one.
package notify

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Available reports whether desktop notifications can be shown on this platform.
func Available() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	_, err := exec.LookPath("osascript")
	return err == nil
}

// Notify shows a desktop notification with the given title and message.
// On unsupported platforms (or when osascript is missing) it returns nil
// without doing anything, so a missing notifier never fails the caller.
func Notify(title, message string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	path, err := exec.LookPath("osascript")
	if err != nil {
		return nil
	}
	script := fmt.Sprintf("display notification %s with title %s", quote(message), quote(title))
	if err := exec.Command(path, "-e", script).Run(); err != nil {
		return fmt.Errorf("osascript: %w", err)
	}
	return nil
}

// quote wraps s in double quotes and escapes the characters that are special
// inside an AppleScript string literal (backslash and double-quote), preventing
// a title or message from breaking out of the script.
func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
