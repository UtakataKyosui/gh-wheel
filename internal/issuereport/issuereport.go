// Package issuereport offers to file a GitHub issue against gh-wheel's own
// repository when the CLI hits an unexpected error or panic.
//
// It never sends anything without explicit interactive consent. Before the
// draft is shown, argument values and token-shaped strings are masked, so the
// preview the user approves is already free of obvious private data. The
// reporter stays silent in non-interactive (piped/CI), --json, and opt-out
// (--no-report / GH_WHEEL_NO_REPORT) contexts.
package issuereport

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"runtime"
	"strings"

	"github.com/UtakataKyosui/gh-wheel/internal/auth"
	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
)

const (
	// ReportOwner and ReportRepo identify the issue tracker that auto-reports
	// are filed against (gh-wheel's own repository).
	ReportOwner = "UtakataKyosui"
	ReportRepo  = "gh-wheel"

	// ReportLabel is applied to every auto-filed issue so they are filterable.
	ReportLabel = "auto-report"

	// OptOutEnv disables the reporter entirely when set to any non-empty value.
	OptOutEnv = "GH_WHEEL_NO_REPORT"

	redactedValue = "‹redacted›"
	redactedToken = "‹token-redacted›"
)

// tokenRe matches GitHub token-shaped strings (classic ghp_/gho_/ghu_/ghs_/ghr_
// and fine-grained github_pat_) so they can be scrubbed from the issue body.
var tokenRe = regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{16,}|github_pat_[A-Za-z0-9_]{20,}`)

// Context holds the diagnostic data captured for one failure. Privacy handling
// differs per field: Args must be pre-redacted by the caller via RedactArgs,
// while ErrMessage and Stack are only token-scrubbed by BuildIssue (GitHub
// token-shaped substrings removed) — their remaining text is shown verbatim in
// the preview that the user must approve before anything is sent.
type Context struct {
	Version     string
	CommandPath string
	Args        []string // pre-redacted by RedactArgs
	GoOS        string
	GoArch      string
	GoVersion   string
	GHVersion   string
	ErrCode     string
	ErrMessage  string
	Stack       string // non-empty only for panics
	IsPanic     bool
}

// Suppressed reports whether the reporter must stay silent regardless of the
// error: machine-readable output (--json), a non-interactive session, or an
// explicit opt-out.
func Suppressed(asJSON, interactive, optOut bool) bool {
	return asJSON || !interactive || optOut
}

// Reportable reports whether err is an unexpected failure worth offering to
// file. Only internal errors (cliexit.ErrCodeGeneral) qualify; auth, usage,
// validation, and API errors are user or environment problems, not bugs.
// A non-structured error is treated as internal.
func Reportable(err error) bool {
	if err == nil {
		return false
	}
	var ce *cliexit.Error
	if errors.As(err, &ce) {
		return ce.Code == cliexit.ErrCodeGeneral
	}
	return true
}

// RedactArgs returns a copy of args with every value masked. Flag tokens
// (-x, --flag) are preserved so the command shape is still legible, but flag
// values and positional arguments are replaced with redactedValue:
//
//	["--body", "secret"]      → ["--body", "‹redacted›"]
//	["--body=secret"]         → ["--body=‹redacted›"]
//	["-R", "org/private"]     → ["-R", "‹redacted›"]
//	["123"]                   → ["‹redacted›"]
//	["--", "--token", "x"]    → ["--", "‹redacted›", "‹redacted›"]
//
// A standalone "--" ends flag parsing: every following token is a positional
// value and is redacted even if it begins with "-" (so secrets passed after
// "--" are not mistaken for flags). Because flag arity is unknown here, every
// non-flag token is conservatively redacted; this can mask a boolean flag's
// unrelated positional, which is the safe direction for privacy.
func RedactArgs(args []string) []string {
	out := make([]string, len(args))
	positionalOnly := false
	for i, a := range args {
		switch {
		case positionalOnly:
			// Everything after a standalone "--" is a positional value.
			out[i] = redactedValue
		case a == "--":
			out[i] = a
			positionalOnly = true
		case strings.HasPrefix(a, "--") && strings.Contains(a, "="):
			eq := strings.IndexByte(a, '=')
			out[i] = a[:eq+1] + redactedValue
		case a == "-":
			out[i] = a // stdin marker, carries no value
		case strings.HasPrefix(a, "-"):
			out[i] = a // flag token, no inline value
		default:
			out[i] = redactedValue
		}
	}
	return out
}

// scrubTokens replaces any GitHub token-shaped substrings with redactedToken.
func scrubTokens(s string) string {
	return tokenRe.ReplaceAllString(s, redactedToken)
}

// Signature is a short, stable description of the failure class used both in
// the issue title and for dedup search. It deliberately avoids backticks so it
// is cleanly matchable by GitHub's in:title search.
func (c Context) Signature() string {
	code := c.ErrCode
	if code == "" {
		code = "PANIC"
	}
	cmd := c.CommandPath
	if cmd == "" {
		cmd = "wheel"
	}
	return fmt.Sprintf("%s in %s", code, cmd)
}

// BuildIssue assembles the issue title and body from c. The body is passed
// through scrubTokens as a final safety net for token-shaped strings that may
// appear in error messages or stack traces.
func BuildIssue(c Context) (title, body string) {
	kind := "予期しないエラー"
	if c.IsPanic {
		kind = "panic"
	}
	title = "[auto-report] " + c.Signature()

	var b strings.Builder
	fmt.Fprintf(&b, "## 自動レポート\n\ngh-wheel 実行中に%sが発生しました。CLI が自動生成したレポートです。\n\n", kind)

	fmt.Fprintf(&b, "## 環境\n\n| 項目 | 値 |\n|---|---|\n")
	fmt.Fprintf(&b, "| gh-wheel | `%s` |\n", emptyDash(c.Version))
	fmt.Fprintf(&b, "| OS / Arch | `%s/%s` |\n", c.GoOS, c.GoArch)
	fmt.Fprintf(&b, "| Go | `%s` |\n", emptyDash(c.GoVersion))
	if c.GHVersion != "" {
		fmt.Fprintf(&b, "| gh CLI | `%s` |\n", c.GHVersion)
	}

	fmt.Fprintf(&b, "\n## コマンド\n\n```\n%s %s\n```\n", emptyDash(c.CommandPath), strings.Join(c.Args, " "))
	fmt.Fprintf(&b, "\n> 引数の値はプライバシー保護のため `%s` にマスクされています。\n", redactedValue)

	fmt.Fprintf(&b, "\n## エラー\n\n- code: `%s`\n", emptyDash(c.ErrCode))
	// Error messages and stack traces are arbitrary text that may itself contain
	// triple backticks; wrap them in 4-backtick fences so the surrounding
	// Markdown does not break.
	if c.ErrMessage != "" {
		fmt.Fprintf(&b, "- message:\n\n````\n%s\n````\n", c.ErrMessage)
	}
	if c.Stack != "" {
		fmt.Fprintf(&b, "\n## スタックトレース\n\n````\n%s\n````\n", c.Stack)
	}
	return title, scrubTokens(b.String())
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// Offer runs the interactive flow for a returned (non-panic) error: it checks
// suppression and reportability, builds and previews the masked draft, asks for
// consent, dedups, and posts. Failure to file is reported to out but never
// escalated — the original error already determines the process exit code.
func Offer(commandPath string, args []string, version string, err error, asJSON, interactive, optOut bool, in io.Reader, out io.Writer) {
	if Suppressed(asJSON, interactive, optOut) || !Reportable(err) {
		return
	}
	c := baseContext(commandPath, args, version)
	var ce *cliexit.Error
	if errors.As(err, &ce) {
		c.ErrCode = string(ce.Code)
		c.ErrMessage = ce.Message
	} else {
		c.ErrCode = string(cliexit.ErrCodeGeneral)
		c.ErrMessage = err.Error()
	}
	run(c, in, out)
}

// OfferPanic is the panic-path variant. Guards still apply, but the report is
// always built (panics are by definition unexpected) and carries the recovered
// value and stack trace.
func OfferPanic(commandPath string, args []string, version string, recovered any, stack []byte, asJSON, interactive, optOut bool, in io.Reader, out io.Writer) {
	if Suppressed(asJSON, interactive, optOut) {
		return
	}
	c := baseContext(commandPath, args, version)
	c.ErrCode = "PANIC"
	c.ErrMessage = fmt.Sprint(recovered)
	c.Stack = string(stack)
	c.IsPanic = true
	run(c, in, out)
}

// baseContext fills in the fields common to both entry points. The gh version
// is resolved here (after the suppression check) so the extra subprocess only
// runs when a report is actually going to be offered.
func baseContext(commandPath string, args []string, version string) Context {
	return Context{
		Version:     version,
		CommandPath: commandPath,
		Args:        RedactArgs(args),
		GoOS:        runtime.GOOS,
		GoArch:      runtime.GOARCH,
		GoVersion:   runtime.Version(),
		GHVersion:   auth.GHVersionString(),
	}
}

// run previews the draft, prompts for consent, then dedups and submits.
func run(c Context, in io.Reader, out io.Writer) {
	title, body := BuildIssue(c)

	fmt.Fprintf(out, "\n―― 予期しない問題が発生しました ――\n")
	fmt.Fprintf(out, "以下の内容で gh-wheel に Issue 下書きを作成しました（引数・トークンはマスク済み）:\n\n")
	fmt.Fprintf(out, "title: %s\n\n%s\n", title, body)

	if !confirm(in, out) {
		fmt.Fprintln(out, "起票をスキップしました。")
		return
	}

	client, err := ghclient.NewForRepo(ReportOwner, ReportRepo)
	if err != nil {
		fmt.Fprintf(out, "Issue 起票に失敗しました: %v\n", err)
		return
	}

	if existing, err := findExisting(client, c.Signature()); err == nil && existing != "" {
		fmt.Fprintf(out, "同じ内容の既存 Issue があります。新規作成はスキップしました: %s\n", existing)
		return
	}

	issueURL, err := submit(client, title, body)
	if err != nil {
		fmt.Fprintf(out, "Issue 起票に失敗しました: %v\n", err)
		return
	}
	fmt.Fprintf(out, "Issue を作成しました: %s\n", issueURL)
}

// confirm prints a yes/no prompt and returns true only for an explicit
// y / yes (case-insensitive). The default (empty input or anything else) is no.
func confirm(in io.Reader, out io.Writer) bool {
	fmt.Fprint(out, "gh-wheel に Issue を起票しますか？ [y/N]: ")
	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		return false
	}
	ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return ans == "y" || ans == "yes"
}

// findExisting searches open auto-report issues whose title contains sig.
// Returns the html_url of the first match, or "" when none exist.
func findExisting(c *ghclient.Client, sig string) (string, error) {
	q := fmt.Sprintf(`repo:%s/%s is:issue is:open label:%s in:title %q`,
		ReportOwner, ReportRepo, ReportLabel, sig)
	var resp struct {
		Items []struct {
			HTMLURL string `json:"html_url"`
		} `json:"items"`
	}
	if err := c.Get("search/issues?q="+url.QueryEscape(q), &resp); err != nil {
		return "", err
	}
	if len(resp.Items) > 0 {
		return resp.Items[0].HTMLURL, nil
	}
	return "", nil
}

// submit creates the issue and returns its html_url.
func submit(c *ghclient.Client, title, body string) (string, error) {
	payload := map[string]any{
		"title":  title,
		"body":   body,
		"labels": []string{ReportLabel},
	}
	var resp struct {
		HTMLURL string `json:"html_url"`
	}
	if err := c.RepoPost("issues", payload, &resp); err != nil {
		return "", err
	}
	return resp.HTMLURL, nil
}
