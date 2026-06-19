package task

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
	"github.com/UtakataKyosui/gh-wheel/internal/jsonout"
)

// planItem types, in descending priority order.
const (
	typeReview     = "review"      // a PR awaiting your review (unblock others)
	typePRMerge    = "pr-merge"    // your approved PR, ready to merge
	typePRFix      = "pr-fix"      // your PR with outstanding changes requested
	typeInProgress = "in-progress" // an Issue assigned to you, continue
	typeNew        = "new"         // a fresh approachable Issue to pick up
)

// planItem is a single entry in the daily plan.
type planItem struct {
	Type            string   `json:"type"`
	Number          int      `json:"number"`
	Title           string   `json:"title"`
	URL             string   `json:"url"`
	Labels          []string `json:"labels"`
	EstimateMinutes int      `json:"estimate_minutes"`
	Reason          string   `json:"reason"`
	Score           int      `json:"score,omitempty"` // new candidates only
}

// effortConfig holds the per-category effort estimates (minutes).
type effortConfig struct {
	review int
	merge  int
	prFix  int
	issue  int // in-progress and new
}

// todayParams echoes the inputs that shaped the plan.
type todayParams struct {
	BudgetMinutes       int `json:"budget_minutes"`
	ReviewEffortMinutes int `json:"review_effort_minutes"`
	MergeEffortMinutes  int `json:"merge_effort_minutes"`
	PRFixEffortMinutes  int `json:"pr_fix_effort_minutes"`
	IssueEffortMinutes  int `json:"issue_effort_minutes"`
}

// todayResult is the JSON output for `gh wheel task today`.
type todayResult struct {
	SchemaVersion         string      `json:"schema_version"`
	Kind                  string      `json:"kind"`
	Repository            string      `json:"repository"`
	User                  string      `json:"user"`
	GeneratedAt           time.Time   `json:"generated_at"`
	Params                todayParams `json:"params"`
	Plan                  []planItem  `json:"plan"`
	Deferred              []planItem  `json:"deferred"`
	TotalEstimatedMinutes int         `json:"total_estimated_minutes"`
	OverBudget            bool        `json:"over_budget"`
	Truncated             bool        `json:"truncated"`
}

func newTodayCmd() *cobra.Command {
	var (
		flagBudget       string
		flagReviewEffort int
		flagMergeEffort  int
		flagPRFixEffort  int
		flagIssueEffort  int
		flagNoNew        bool
		flagNoBlockers   bool
	)

	cmd := &cobra.Command{
		Use:   "today",
		Short: "Plan today's tasks within a time budget",
		Long: `Build a read-only plan of what to work on today, fitted to a time budget.

It gathers everything on your plate — PRs awaiting your review, your own PRs
that are approved (ready to merge) or have changes requested, Issues assigned to
you, and (if budget remains) fresh approachable Issues — then ranks them by
priority:

  review > pr-merge > pr-fix > in-progress > new

Each item is given a per-category effort estimate and the budget is filled
greedily in priority order: items that fit go into the plan, the rest are
deferred. The single highest-priority item is always included even if it alone
exceeds the budget (so the plan is never empty when there is work).

This command is READ-ONLY: it never assigns, labels, or merges anything, so
--dry-run produces identical output. Honours -R/--json/--jq.

Examples:
  gh wheel task today
  gh wheel task today --budget 4h --json
  gh wheel task today --no-new --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			budget, err := time.ParseDuration(flagBudget)
			if err != nil || budget <= 0 {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
					fmt.Errorf("invalid --budget %q: must be a positive duration like 6h, 90m, or 4h30m", flagBudget))
			}
			budgetMin := int(budget.Minutes())

			flagRepo, _ := cmd.Flags().GetString("repo")
			jsonOut, _ := cmd.Flags().GetBool("json")
			jqExpr, _ := cmd.Flags().GetString("jq")

			c, err := ghclient.New(flagRepo)
			if err != nil {
				return err
			}
			login, err := c.CurrentUser()
			if err != nil {
				return err
			}
			repo := fmt.Sprintf("%s/%s", c.Owner(), c.Name())

			// One fetch covers review-requested + authored PRs (with review state)
			// and Issues assigned to you. WithReviews is required to classify own
			// PRs and to confirm review-requested items are still pending.
			tr, err := fetch(c, login, fetchOpts{
				State:         "open",
				WithIssues:    true,
				WithReviews:   true,
				IncludeDrafts: false,
			})
			if err != nil {
				return err
			}

			cfg := effortConfig{
				review: flagReviewEffort,
				merge:  flagMergeEffort,
				prFix:  flagPRFixEffort,
				issue:  flagIssueEffort,
			}
			items := buildPlanItems(tr, login, cfg)

			// Effort already claimed by PRs + in-progress. Only reach for fresh
			// candidates (and their per-Issue blocker GraphQL) when budget remains.
			consumed := 0
			for _, it := range items {
				consumed += it.EstimateMinutes
			}

			truncatedNew := false
			if !flagNoNew && consumed < budgetMin {
				var checker hasOpenSubIssueFn
				if !flagNoBlockers {
					gql, err := c.GraphQL()
					if err != nil {
						return err
					}
					checker = makeSubIssueChecker(gql, c.Owner(), c.Name())
				}
				// limit 0: no cap — the budget fill below decides how many to keep.
				cands, err := gatherCandidates(c, repo, "", flagNoBlockers, checker, 0)
				if err != nil {
					return err
				}
				if len(cands) >= 100 {
					truncatedNew = true
				}
				items = append(items, candidateItems(cands, cfg.issue)...)
			}

			plan, deferred, total, over := fillBudget(items, budgetMin)

			result := todayResult{
				SchemaVersion: "v1",
				Kind:          "task_today_result",
				Repository:    repo,
				User:          login,
				GeneratedAt:   tr.FetchedAt,
				Params: todayParams{
					BudgetMinutes:       budgetMin,
					ReviewEffortMinutes: cfg.review,
					MergeEffortMinutes:  cfg.merge,
					PRFixEffortMinutes:  cfg.prFix,
					IssueEffortMinutes:  cfg.issue,
				},
				Plan:                  plan,
				Deferred:              deferred,
				TotalEstimatedMinutes: total,
				OverBudget:            over,
				// Search caps each query at 100 results with no pagination, so a
				// full page is a best-effort signal that the plan was computed
				// over a truncated set.
				Truncated: truncatedNew || len(tr.PRs) >= 100 || len(tr.Issues) >= 100,
			}

			if jsonOut {
				return jsonout.Print(result, jqExpr)
			}
			printToday(os.Stdout, result)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagBudget, "budget", "6h", "Time budget for today (duration: 6h, 90m, 4h30m)")
	cmd.Flags().IntVar(&flagReviewEffort, "review-effort", 20, "Estimated minutes per PR review")
	cmd.Flags().IntVar(&flagMergeEffort, "merge-effort", 10, "Estimated minutes to merge an approved PR")
	cmd.Flags().IntVar(&flagPRFixEffort, "pr-effort", 45, "Estimated minutes to address changes requested on your PR")
	cmd.Flags().IntVar(&flagIssueEffort, "issue-effort", 90, "Estimated minutes per Issue (in-progress or new)")
	cmd.Flags().BoolVar(&flagNoNew, "no-new", false, "Do not include fresh approachable Issues")
	cmd.Flags().BoolVar(&flagNoBlockers, "no-blockers", false, "Skip blocker filtering for new Issues (faster; no GraphQL queries)")
	return cmd
}

// ─── classification ──────────────────────────────────────────────────────────

// buildPlanItems classifies PRs and assigned Issues into prioritised planItems.
// Each PR maps to at most one item (deduped by number); category priority order
// is review > pr-merge > pr-fix > in-progress. New candidates are appended by
// the caller.
func buildPlanItems(tr *TaskResult, login string, cfg effortConfig) []planItem {
	var reviews, merges, fixes, inProgress []planItem
	seen := make(map[int]bool, len(tr.PRs))

	for _, pr := range tr.PRs {
		if seen[pr.Number] {
			continue
		}
		isAuthor := containsString(pr.Categories, "author")
		isReviewer := containsString(pr.Categories, "review-requested")

		// Your own PR: classify by human review state. Takes precedence over
		// the reviewer bucket (you never review your own PR).
		if isAuthor {
			switch classifyHumanReview(pr.Reviews, pr.LatestCommitAt) {
			case reviewApproved:
				merges = append(merges, prItem(typePRMerge, pr, cfg.merge, "approved — ready to merge"))
				seen[pr.Number] = true
				continue
			case reviewChangesRequested:
				fixes = append(fixes, prItem(typePRFix, pr, cfg.prFix, "changes requested"))
				seen[pr.Number] = true
				continue
			default:
				// none / addressed / commented → no actionable own-PR work today.
			}
		}

		// PR awaiting your review. Drop it if you have already reviewed since the
		// latest commit (covers self-review and search-index lag). Team-based
		// review requests are a known v1 limitation (requested-reviewers payload
		// is not fetched).
		if isReviewer && !isAuthor && !alreadyReviewedByYou(pr, login) {
			reviews = append(reviews, prItem(typeReview, pr, cfg.review, "review requested from you"))
			seen[pr.Number] = true
		}
	}

	// Issues assigned to you (open). Disjoint from `new` candidates by
	// construction: buildCandidateQuery forces no:assignee, this list is
	// assignee:@me.
	for _, iss := range tr.Issues {
		inProgress = append(inProgress, planItem{
			Type:            typeInProgress,
			Number:          iss.Number,
			Title:           iss.Title,
			URL:             iss.URL,
			Labels:          iss.Labels,
			EstimateMinutes: cfg.issue,
			Reason:          "assigned to you — continue",
		})
	}

	items := make([]planItem, 0, len(reviews)+len(merges)+len(fixes)+len(inProgress))
	items = append(items, reviews...)
	items = append(items, merges...)
	items = append(items, fixes...)
	items = append(items, inProgress...)
	return items
}

// candidateItems converts ranked new-Issue candidates into planItems.
func candidateItems(cands []candidate, est int) []planItem {
	items := make([]planItem, 0, len(cands))
	for _, c := range cands {
		reason := "approachable issue"
		if c.Score != 0 {
			reason = fmt.Sprintf("approachable issue (score %d)", c.Score)
		}
		items = append(items, planItem{
			Type:            typeNew,
			Number:          c.Number,
			Title:           c.Title,
			URL:             c.URL,
			Labels:          c.Labels,
			EstimateMinutes: est,
			Reason:          reason,
			Score:           c.Score,
		})
	}
	return items
}

func prItem(itemType string, pr PR, est int, reason string) planItem {
	return planItem{
		Type:            itemType,
		Number:          pr.Number,
		Title:           pr.Title,
		URL:             pr.URL,
		Labels:          pr.Labels,
		EstimateMinutes: est,
		Reason:          reason,
	}
}

// alreadyReviewedByYou reports whether login submitted a review after the PR's
// latest commit (i.e. you have already seen the current state).
func alreadyReviewedByYou(pr PR, login string) bool {
	// DISMISSED (your review was dismissed → re-review needed) and PENDING (an
	// unsubmitted draft) do not count as having reviewed the current state.
	reviewed := func(r Review) bool {
		return r.Author == login && r.State != "DISMISSED" && r.State != "PENDING"
	}
	if pr.LatestCommitAt.IsZero() {
		for _, r := range pr.Reviews {
			if reviewed(r) {
				return true
			}
		}
		return false
	}
	for _, r := range pr.Reviews {
		if reviewed(r) && r.SubmittedAt.After(pr.LatestCommitAt) {
			return true
		}
	}
	return false
}

func containsString(haystack []string, want string) bool {
	for _, h := range haystack {
		if h == want {
			return true
		}
	}
	return false
}

// ─── budget fill ─────────────────────────────────────────────────────────────

// fillBudget greedily selects items in priority order until the budget is spent.
// The first item is always selected (plan is never empty when work exists); when
// it alone exceeds the budget, over is true. Once an item overflows, it and all
// subsequent items are deferred — no scanning ahead for smaller fits.
func fillBudget(items []planItem, budgetMin int) (plan, deferred []planItem, total int, over bool) {
	plan = []planItem{}
	deferred = []planItem{}
	overflowed := false
	for i, it := range items {
		switch {
		case overflowed:
			deferred = append(deferred, it)
		case i == 0:
			plan = append(plan, it)
			total += it.EstimateMinutes
			if total > budgetMin {
				over = true
				overflowed = true
			}
		case total+it.EstimateMinutes <= budgetMin:
			plan = append(plan, it)
			total += it.EstimateMinutes
		default:
			overflowed = true
			deferred = append(deferred, it)
		}
	}
	return plan, deferred, total, over
}

// ─── text output ─────────────────────────────────────────────────────────────

func printToday(w io.Writer, r todayResult) {
	fmt.Fprintf(w, "今日のタスク — 予算 %s / 見積 %s\n",
		fmtMinutes(r.Params.BudgetMinutes), fmtMinutes(r.TotalEstimatedMinutes))
	if r.OverBudget {
		fmt.Fprintln(w, "⚠ 最優先タスク単体で予算を超えています。")
	}
	if r.Truncated {
		fmt.Fprintln(w, "⚠ 候補が多く一部のみで計画しています（検索結果は最大100件）。")
	}

	if len(r.Plan) == 0 {
		fmt.Fprintln(w, "今日やるべきタスクが見つかりません。")
		return
	}

	fmt.Fprintln(w)
	for _, it := range r.Plan {
		printPlanItem(w, it)
	}

	if len(r.Deferred) > 0 {
		fmt.Fprintln(w, "\n── 予算外 (deferred) ──")
		for _, it := range r.Deferred {
			printPlanItem(w, it)
		}
	}
}

func printPlanItem(w io.Writer, it planItem) {
	fmt.Fprintf(w, "%s #%d %s  [%s · %s]\n",
		typeIcon(it.Type), it.Number, it.Title, it.Type, fmtMinutes(it.EstimateMinutes))
	if it.Reason != "" {
		fmt.Fprintf(w, "    %s — %s\n", it.Reason, it.URL)
	}
}

func typeIcon(t string) string {
	switch t {
	case typeReview:
		return "👀"
	case typePRMerge:
		return "🚀"
	case typePRFix:
		return "🔧"
	case typeInProgress:
		return "🚧"
	case typeNew:
		return "✨"
	default:
		return "•"
	}
}

func fmtMinutes(m int) string {
	h := m / 60
	min := m % 60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", min)
	case min == 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dh%dm", h, min)
	}
}
