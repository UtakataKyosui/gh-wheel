package task

import (
	"fmt"
	"io"
)

// badge converts categories to a compact display tag: [A], [R], or [AR].
func badge(cats []string) string {
	var a, r bool
	for _, c := range cats {
		switch c {
		case "author":
			a = true
		case "review-requested":
			r = true
		}
	}
	switch {
	case a && r:
		return "[AR]"
	case a:
		return "[A] "
	case r:
		return "[R] "
	default:
		return "    "
	}
}

// printTable writes a human-readable summary of a TaskResult to w.
func printTable(w io.Writer, result *TaskResult) {
	if len(result.PRs) == 0 && len(result.Issues) == 0 {
		fmt.Fprintln(w, "No results found.")
		return
	}

	if len(result.PRs) > 0 {
		fmt.Fprintln(w, "Pull Requests")
		fmt.Fprintln(w, "─────────────")
		for _, pr := range result.PRs {
			stateStr := pr.State
			if pr.IsDraft {
				stateStr = "draft"
			}
			fmt.Fprintf(w, "  PR #%-5d  %s  %-6s  %s\n",
				pr.Number, badge(pr.Categories), stateStr, pr.Title)
		}
	}

	if len(result.Issues) > 0 {
		if len(result.PRs) > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, "Issues")
		fmt.Fprintln(w, "──────")
		for _, iss := range result.Issues {
			fmt.Fprintf(w, "  Issue #%-5d  %-6s  %s\n",
				iss.Number, iss.State, iss.Title)
		}
	}
}
