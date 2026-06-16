package task

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
)

// issueState is the GitHub Issues API response shape used by the close subcommand.
type issueState struct {
	State   string `json:"state"`
	Title   string `json:"title"`
	HTMLURL string `json:"html_url"`
}

// newCloseCmd returns the `gh wheel task close <N>` subcommand.
func newCloseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "close <N>",
		Short: "Close a PR or Issue by number",
		Long: `Close the PR or Issue with the given number.

By default the command prints the item's title, state, and URL, then asks you
to re-enter the number as a confirmation before closing.  Pass --json to skip
the confirmation and close immediately.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := strconv.Atoi(args[0])
			if err != nil || n <= 0 {
				return fmt.Errorf("invalid number %q: must be a positive integer", args[0])
			}

			flagRepo, _ := cmd.Flags().GetString("repo")
			jsonMode, _ := cmd.Flags().GetBool("json")

			c, err := ghclient.New(flagRepo)
			if err != nil {
				return err
			}

			return confirmAndClose(c, n, jsonMode, os.Stdin, os.Stdout)
		},
	}
	return cmd
}

// confirmAndClose fetches the issue/PR state, optionally prompts for confirmation,
// and closes the item by patching its state to "closed".
//
// Parameters:
//   - c:        GitHub API client
//   - n:        issue or PR number
//   - jsonMode: when true, skip the interactive confirmation prompt
//   - in:       reader for the confirmation input (normally os.Stdin)
//   - out:      writer for success messages (normally os.Stdout)
func confirmAndClose(c *ghclient.Client, n int, jsonMode bool, in io.Reader, out io.Writer) error {
	// 1. Fetch current state.
	var state issueState
	if err := c.RepoGet(fmt.Sprintf("issues/%d", n), &state); err != nil {
		return err
	}

	// 2. Short-circuit if already closed.
	if state.State == "closed" {
		fmt.Fprintln(out, "Already closed.")
		return nil
	}

	// 3. Show info to stderr.
	fmt.Fprintf(os.Stderr, "#%d  %s\n", n, state.Title)
	fmt.Fprintf(os.Stderr, "state: %s\n", state.State)
	fmt.Fprintf(os.Stderr, "url:   %s\n", state.HTMLURL)

	// 4. Prompt for confirmation unless --json was given.
	if !jsonMode {
		fmt.Fprintf(os.Stderr, "番号 %d を再入力して確認: ", n)

		scanner := bufio.NewScanner(in)
		scanner.Scan()
		input := strings.TrimSpace(scanner.Text())

		got, err := strconv.Atoi(input)
		if err != nil || got != n {
			return fmt.Errorf("confirmation mismatch: expected %d, got %q", n, input)
		}
	}

	// 5. PATCH state to closed.
	var updated issueState
	if err := c.RepoPatch(fmt.Sprintf("issues/%d", n), map[string]string{"state": "closed"}, &updated); err != nil {
		return err
	}

	fmt.Fprintf(out, "#%d closed.\n", n)
	return nil
}
