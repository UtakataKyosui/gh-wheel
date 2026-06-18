package task

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	ghapi "github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
	"github.com/UtakataKyosui/gh-wheel/internal/graph/graphql"
	"github.com/UtakataKyosui/gh-wheel/internal/jsonout"
)

// nonActionableLabels marks Issues that are not directly workable: aggregators
// (epic) and triage outcomes. They are excluded from "next" candidates.
var nonActionableLabels = []string{"epic", "wontfix", "invalid", "duplicate", "question"}

// candidate is a ranked, ready-to-start Issue.
type candidate struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	URL    string   `json:"url"`
	Labels []string `json:"labels"`
	Score  int      `json:"score"`
	Ready  bool     `json:"ready"`
}

// assignment records a completed self-assignment.
type assignment struct {
	Number   int    `json:"number"`
	Assignee string `json:"assignee"`
	URL      string `json:"url"`
}

// nextResult is the JSON output for `gh wheel task next`.
type nextResult struct {
	SchemaVersion string      `json:"schema_version"`
	Kind          string      `json:"kind"`
	Repository    string      `json:"repository"`
	Candidates    []candidate `json:"candidates"`
	Assigned      *assignment `json:"assigned"`
	WouldAssign   *candidate  `json:"would_assign,omitempty"`
}

func newNextCmd() *cobra.Command {
	var (
		flagList       bool
		flagYes        bool
		flagNoBlockers bool
		flagLimit      int
		flagLabel      string
	)

	cmd := &cobra.Command{
		Use:   "next [N]",
		Short: "Find an approachable unstarted Issue and self-assign",
		Long: `Find an open, unassigned Issue that is ready to start and assign it to you.

"Ready to start" means: open, unassigned, not labelled epic/wontfix/invalid/
duplicate/question, and (unless --no-blockers) not already linked to a PR and
not an aggregator with open sub-issues. Candidates are ranked, preferring
"good first issue" / "help wanted" and de-prioritising "priority:low".

By default the top candidate is shown and you re-enter its number to confirm
before it is assigned. Pass [N] to target a specific Issue, --list to only show
candidates, --dry-run to preview, or --json/--yes to skip the confirmation.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			flagRepo, _ := cmd.Flags().GetString("repo")
			jsonOut, _ := cmd.Flags().GetBool("json")
			jqExpr, _ := cmd.Flags().GetString("jq")
			dryRun, _ := cmd.Root().PersistentFlags().GetBool("dry-run")

			c, err := ghclient.New(flagRepo)
			if err != nil {
				return err
			}
			repo := fmt.Sprintf("%s/%s", c.Owner(), c.Name())

			// Explicit [N]: assign a specific Issue, skipping discovery.
			if len(args) == 1 {
				n, err := strconv.Atoi(args[0])
				if err != nil || n <= 0 {
					return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
						fmt.Errorf("invalid issue number %q: must be a positive integer", args[0]))
				}
				return assignExplicit(c, repo, n, jsonOut, jqExpr, dryRun, flagYes)
			}

			// Discover candidates.
			var checker hasOpenSubIssueFn
			if !flagNoBlockers {
				gql, err := c.GraphQL()
				if err != nil {
					return err
				}
				checker = makeSubIssueChecker(gql, c.Owner(), c.Name())
			}
			cands, err := gatherCandidates(c, repo, flagLabel, flagNoBlockers, checker, flagLimit)
			if err != nil {
				return err
			}
			if cands == nil {
				cands = []candidate{}
			}

			result := nextResult{
				SchemaVersion: "v1",
				Kind:          "task_next_result",
				Repository:    repo,
				Candidates:    cands,
			}

			if len(cands) == 0 {
				if jsonOut {
					return jsonout.Print(result, jqExpr)
				}
				fmt.Fprintln(os.Stdout, "No approachable issues found.")
				return nil
			}

			top := cands[0]

			if dryRun {
				result.Kind = "task_next_preview"
				result.WouldAssign = &top
				if jsonOut {
					return jsonout.Print(result, jqExpr)
				}
				printCandidates(os.Stderr, cands)
				fmt.Fprintf(os.Stdout, "[dry-run] would assign #%d to you: %s (%s)\n", top.Number, top.Title, top.URL)
				return nil
			}

			if flagList {
				if jsonOut {
					return jsonout.Print(result, jqExpr)
				}
				printCandidates(os.Stdout, cands)
				return nil
			}

			login, err := c.CurrentUser()
			if err != nil {
				return err
			}
			printCandidates(os.Stderr, cands)
			asg, err := confirmAndAssign(c, top, login, jsonOut || flagYes, os.Stdin, statusWriter(jsonOut))
			if err != nil {
				return err
			}
			result.Assigned = asg
			if jsonOut {
				return jsonout.Print(result, jqExpr)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagList, "list", false, "List candidates without assigning")
	cmd.Flags().BoolVar(&flagYes, "yes", false, "Skip the confirmation prompt before assigning")
	cmd.Flags().BoolVar(&flagNoBlockers, "no-blockers", false, "Skip blocker filtering (faster; no GraphQL queries)")
	cmd.Flags().IntVar(&flagLimit, "limit", 5, "Maximum number of candidates to show")
	cmd.Flags().StringVar(&flagLabel, "label", "", "Restrict candidates to a label")
	return cmd
}

// ─── candidate discovery ───────────────────────────────────────────────────────

// buildCandidateQuery builds the search/issues query for ready-to-start Issues.
func buildCandidateQuery(repo, label string, noBlockers bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "is:issue is:open no:assignee repo:%s", repo)
	for _, l := range nonActionableLabels {
		fmt.Fprintf(&b, " -label:%s", l)
	}
	if !noBlockers {
		b.WriteString(" -linked:pr")
	}
	if label != "" {
		fmt.Fprintf(&b, " label:%q", label)
	}
	return b.String()
}

func isExcludedLabel(labels []string) bool {
	for _, l := range labels {
		for _, ex := range nonActionableLabels {
			if strings.EqualFold(l, ex) {
				return true
			}
		}
	}
	return false
}

// scoreLabels ranks a candidate: prefer good-first/help-wanted, demote low priority.
func scoreLabels(labels []string) int {
	score := 0
	for _, l := range labels {
		switch strings.ToLower(l) {
		case "good first issue":
			score += 2
		case "help wanted":
			score++
		case "priority:low":
			score -= 2
		}
	}
	return score
}

// rankCandidates sorts by score descending, then by issue number ascending
// (older first) for deterministic, stable output.
func rankCandidates(cands []candidate) []candidate {
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].Score != cands[j].Score {
			return cands[i].Score > cands[j].Score
		}
		return cands[i].Number < cands[j].Number
	})
	return cands
}

// hasOpenSubIssueFn reports whether Issue num is an aggregator with at least one
// open sub-issue. Injected so the discovery pipeline is testable without GraphQL.
type hasOpenSubIssueFn func(num int) (bool, error)

func makeSubIssueChecker(gql *ghapi.GraphQLClient, owner, repo string) hasOpenSubIssueFn {
	return func(num int) (bool, error) {
		res, err := graphql.QuerySubIssues(gql, owner, repo, num)
		if err != nil {
			return false, cliexit.NewAPI(cliexit.ErrCodeAPI, fmt.Errorf("sub-issues for #%d: %w", num, err))
		}
		for _, child := range res.Children {
			if strings.EqualFold(child.State, "OPEN") {
				return true, nil
			}
		}
		return false, nil
	}
}

// gatherCandidates searches, scores, and ranks ready-to-start Issues, then
// applies blocker filtering lazily from the top: the per-Issue sub-issue check
// runs only until `limit` non-blocked candidates are found (limit <= 0 means no
// cap → every candidate is checked). This avoids one GraphQL call per search hit
// when only the top few are shown.
// check may be nil when noBlockers is true.
func gatherCandidates(c *ghclient.Client, repo, label string, noBlockers bool, check hasOpenSubIssueFn, limit int) ([]candidate, error) {
	items, err := searchItems(c, buildCandidateQuery(repo, label, noBlockers))
	if err != nil {
		return nil, err
	}

	var ranked []candidate
	for _, it := range items {
		labels := make([]string, len(it.Labels))
		for i, l := range it.Labels {
			labels[i] = l.Name
		}
		if isExcludedLabel(labels) { // safety net; the query already excludes these
			continue
		}
		ranked = append(ranked, candidate{
			Number: it.Number,
			Title:  it.Title,
			URL:    it.HTMLURL,
			Labels: labels,
			Score:  scoreLabels(labels),
			Ready:  true,
		})
	}
	ranked = rankCandidates(ranked)

	// No blocker filtering: just cap to limit.
	if noBlockers || check == nil {
		return capCandidates(ranked, limit), nil
	}

	// Blocker-check ranked candidates from the top, stopping once `limit`
	// non-blocked ones are collected.
	var out []candidate
	for _, cand := range ranked {
		blocked, err := check(cand.Number)
		if err != nil {
			return nil, err
		}
		if blocked {
			continue
		}
		out = append(out, cand)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// capCandidates truncates cands to limit; limit <= 0 means no cap.
func capCandidates(cands []candidate, limit int) []candidate {
	if limit > 0 && len(cands) > limit {
		return cands[:limit]
	}
	return cands
}

// ─── assignment ────────────────────────────────────────────────────────────────

func assignIssue(c *ghclient.Client, num int, login string) error {
	var resp struct{}
	return c.RepoPost(fmt.Sprintf("issues/%d/assignees", num), map[string][]string{"assignees": {login}}, &resp)
}

// confirmAndAssign optionally prompts for confirmation (re-enter the number),
// then assigns the Issue to login.
func confirmAndAssign(c *ghclient.Client, cand candidate, login string, skipConfirm bool, in io.Reader, out io.Writer) (*assignment, error) {
	if !skipConfirm {
		fmt.Fprintf(os.Stderr, "Assign #%d to @%s?\n", cand.Number, login)
		fmt.Fprintf(os.Stderr, "  %s\n  %s\n", cand.Title, cand.URL)
		fmt.Fprintf(os.Stderr, "Re-enter %d to confirm: ", cand.Number)

		scanner := bufio.NewScanner(in)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return nil, cliexit.NewGeneral(fmt.Errorf("reading confirmation: %w", err))
			}
			return nil, cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
				fmt.Errorf("no confirmation input received"))
		}
		got := strings.TrimSpace(scanner.Text())
		n, err := strconv.Atoi(got)
		if err != nil || n != cand.Number {
			return nil, cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
				fmt.Errorf("confirmation mismatch: expected %d, got %q", cand.Number, got))
		}
	}

	if err := assignIssue(c, cand.Number, login); err != nil {
		return nil, err
	}
	fmt.Fprintf(out, "#%d assigned to @%s.\n", cand.Number, login)
	return &assignment{Number: cand.Number, Assignee: login, URL: cand.URL}, nil
}

// assignExplicit assigns a specific Issue number provided as a positional arg.
func assignExplicit(c *ghclient.Client, repo string, n int, jsonOut bool, jqExpr string, dryRun, yes bool) error {
	var st struct {
		Title       string    `json:"title"`
		HTMLURL     string    `json:"html_url"`
		State       string    `json:"state"`
		PullRequest *struct{} `json:"pull_request"`
	}
	if err := c.RepoGet(fmt.Sprintf("issues/%d", n), &st); err != nil {
		return err
	}
	// GET issues/{n} returns PRs too (with a pull_request object); refuse them
	// so `task next <PR#>` cannot assign you to a pull request.
	if st.PullRequest != nil {
		return cliexit.NewNotFound(cliexit.ErrCodeNotFound,
			fmt.Errorf("#%d is a pull request, not an issue", n))
	}
	if st.State != "open" {
		// Exit code stays not_found (3) but the machine-readable code reflects
		// the real condition: the Issue exists, it just is not open.
		return cliexit.NewNotFound(cliexit.ErrCodeIssueNotOpen,
			fmt.Errorf("issue #%d is not open (state: %s)", n, st.State))
	}

	// Labels initialised as an empty slice (not nil) so JSON emits [] like the
	// discovered-candidate path, not null.
	cand := candidate{Number: n, Title: st.Title, URL: st.HTMLURL, Labels: []string{}, Ready: true}
	result := nextResult{
		SchemaVersion: "v1",
		Kind:          "task_next_result",
		Repository:    repo,
		Candidates:    []candidate{cand},
	}

	if dryRun {
		result.Kind = "task_next_preview"
		result.WouldAssign = &cand
		if jsonOut {
			return jsonout.Print(result, jqExpr)
		}
		fmt.Fprintf(os.Stdout, "[dry-run] would assign #%d to you: %s (%s)\n", n, cand.Title, cand.URL)
		return nil
	}

	login, err := c.CurrentUser()
	if err != nil {
		return err
	}
	asg, err := confirmAndAssign(c, cand, login, jsonOut || yes, os.Stdin, statusWriter(jsonOut))
	if err != nil {
		return err
	}
	result.Assigned = asg
	if jsonOut {
		return jsonout.Print(result, jqExpr)
	}
	return nil
}

// statusWriter selects the stream for human status/success messages: stderr in
// JSON mode (so stdout carries only the JSON document), stdout otherwise.
func statusWriter(jsonOut bool) io.Writer {
	if jsonOut {
		return os.Stderr
	}
	return os.Stdout
}

// printCandidates writes a human-readable ranked list; the top row is marked.
func printCandidates(w io.Writer, cands []candidate) {
	fmt.Fprintln(w, "Approachable issues:")
	for i, c := range cands {
		marker := "  "
		if i == 0 {
			marker = "→ "
		}
		labels := ""
		if len(c.Labels) > 0 {
			labels = "  [" + strings.Join(c.Labels, ", ") + "]"
		}
		fmt.Fprintf(w, "%s#%d (score %d) %s%s\n", marker, c.Number, c.Score, c.Title, labels)
	}
}
