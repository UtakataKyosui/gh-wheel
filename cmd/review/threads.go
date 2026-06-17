package review

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
	"github.com/UtakataKyosui/gh-wheel/internal/jsonout"
)

const threadsQuery = `
query($owner:String!,$repo:String!,$pr:Int!,$cursor:String){
  repository(owner:$owner,name:$repo){
    pullRequest(number:$pr){
      reviewThreads(first:50,after:$cursor){
        nodes{
          id isResolved isOutdated path line
          comments(first:10){ nodes{ id body author{login} createdAt url } }
        }
        pageInfo{ hasNextPage endCursor }
      }
    }
  }
}
`

type gqlThreadsResp struct {
	Repository struct {
		PullRequest struct {
			ReviewThreads struct {
				Nodes    []reviewThread `json:"nodes"`
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"reviewThreads"`
		} `json:"pullRequest"`
	} `json:"repository"`
}

type reviewThread struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Line       int    `json:"line"`
	IsResolved bool   `json:"isResolved"`
	IsOutdated bool   `json:"isOutdated"`
	Comments   struct {
		Nodes []threadComment `json:"nodes"`
	} `json:"comments"`
}

type threadComment struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	URL       string `json:"url"`
	Author    struct {
		Login string `json:"login"`
	} `json:"author"`
	CreatedAt string `json:"createdAt"`
}

func newThreadsCmd() *cobra.Command {
	var (
		flagRepo string
		flagJSON bool
	)

	cmd := &cobra.Command{
		Use:   "threads <PR>",
		Short: "List unresolved review threads for a pull request",
		Long:  "Fetches all review threads for the given PR and prints the ones that are neither resolved nor outdated.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prNum, err := strconv.Atoi(args[0])
			if err != nil || prNum <= 0 {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
					fmt.Errorf("PR must be a positive integer, got %q", args[0]))
			}

			c, err := ghclient.New(flagRepo)
			if err != nil {
				return err
			}

			all, err := fetchAllThreads(c, prNum)
			if err != nil {
				return err
			}

			unresolved := filterThreads(all)

			if flagJSON {
				result := struct {
					SchemaVersion string         `json:"schema_version"`
					Kind          string         `json:"kind"`
					PR            int            `json:"pr"`
					Threads       []reviewThread `json:"threads"`
				}{
					SchemaVersion: "v1",
					Kind:          "review_threads_result",
					PR:            prNum,
					Threads:       unresolved,
				}
				if result.Threads == nil {
					result.Threads = []reviewThread{}
				}
				jqExpr, _ := cmd.Root().PersistentFlags().GetString("jq")
				return jsonout.Print(result, jqExpr)
			}

			if len(unresolved) == 0 {
				fmt.Println("No unresolved review threads.")
				return nil
			}

			printThreadsTable(os.Stdout, unresolved)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flagRepo, "repo", "R", "", "Repository (OWNER/REPO). Defaults to current directory's repo.")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "Output as JSON")

	return cmd
}

// fetchAllThreads pages through all review threads for prNum and returns them all.
func fetchAllThreads(c *ghclient.Client, prNum int) ([]reviewThread, error) {
	gql, err := c.GraphQL()
	if err != nil {
		return nil, err
	}

	var all []reviewThread
	var cursor *string

	for {
		vars := map[string]interface{}{
			"owner":  c.Owner(),
			"repo":   c.Name(),
			"pr":     prNum,
			"cursor": cursor,
		}

		var result gqlThreadsResp
		if err := gql.Do(threadsQuery, vars, &result); err != nil {
			return nil, cliexit.NewAPI(cliexit.ErrCodeAPI,
				fmt.Errorf("GraphQL query failed: %w", err))
		}

		nodes := result.Repository.PullRequest.ReviewThreads.Nodes
		all = append(all, nodes...)

		pageInfo := result.Repository.PullRequest.ReviewThreads.PageInfo
		if !pageInfo.HasNextPage {
			break
		}
		endCursor := pageInfo.EndCursor
		cursor = &endCursor
	}

	return all, nil
}

// filterThreads returns only threads that are neither resolved nor outdated.
func filterThreads(threads []reviewThread) []reviewThread {
	result := make([]reviewThread, 0, len(threads))
	for _, t := range threads {
		if !t.IsResolved && !t.IsOutdated {
			result = append(result, t)
		}
	}
	return result
}

// truncateComment truncates s to at most limit runes, appending "..." if truncated.
func truncateComment(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "..."
}

// printThreadsTable writes a tab-separated table of unresolved threads to w.
func printThreadsTable(w io.Writer, threads []reviewThread) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tPATH:LINE\tAUTHOR\tCOMMENT")
	fmt.Fprintln(tw, "--\t---------\t------\t-------")

	for _, t := range threads {
		pathLine := fmt.Sprintf("%s:%d", t.Path, t.Line)
		author := ""
		body := ""
		if len(t.Comments.Nodes) > 0 {
			first := t.Comments.Nodes[0]
			author = first.Author.Login
			body = truncateComment(first.Body, 60)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", t.ID, pathLine, author, body)
	}

	tw.Flush()
}
