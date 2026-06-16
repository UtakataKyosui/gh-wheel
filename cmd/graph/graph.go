// Package graph implements the `gh wheel graph` subcommand.
// It fetches GitHub Issue / PR relationship data, builds an in-memory graph,
// and renders it in list, tree, DOT, or JSON format.
package graph

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
	model "github.com/UtakataKyosui/gh-wheel/internal/graph"
	"github.com/UtakataKyosui/gh-wheel/internal/graph/graphql"
	ghapi "github.com/cli/go-gh/v2/pkg/api"
	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"
)

// NewCmd returns the `gh wheel graph` subcommand.
func NewCmd() *cobra.Command {
	var (
		issueNum    int
		depth       int
		label       string
		milestone   string
		noTimeline  bool
		noSubIssues bool
		jqFilter    string
		format      string
	)

	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Visualize GitHub Issue/PR relationship graphs",
		Long: `Fetch and display dependency and reference graphs between Issues and PRs.

Examples:
  # Show the full repository graph as a list
  gh wheel graph

  # Show a BFS subgraph rooted at issue #10, depth 3
  gh wheel graph --issue 10 --depth 3

  # Export graph as DOT for Graphviz
  gh wheel graph --format dot

  # Export graph as JSON
  gh wheel graph --format json

  # Filter by label
  gh wheel graph --label "bug"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			flagRepo, _ := cmd.Root().PersistentFlags().GetString("repo")

			c, err := ghclient.New(flagRepo)
			if err != nil {
				return fmt.Errorf("failed to create GitHub client: %w", err)
			}

			gql, err := c.GraphQL()
			if err != nil {
				return fmt.Errorf("failed to create GraphQL client: %w", err)
			}

			owner := c.Owner()
			repoName := c.Name()

			g, err := buildGraph(gql, owner, repoName, label, milestone, noTimeline, noSubIssues)
			if err != nil {
				return err
			}

			// Apply BFS if --issue is set
			if issueNum != 0 {
				startID := findNodeIDByNumber(g, issueNum)
				if startID == "" {
					return fmt.Errorf("issue #%d not found in graph", issueNum)
				}
				g = g.BFS(startID, depth)
			}

			return renderGraph(cmd.OutOrStdout(), g, format, jqFilter)
		},
	}

	cmd.Flags().IntVar(&issueNum, "issue", 0, "BFS start issue number (0 = fetch whole repo)")
	cmd.Flags().IntVar(&depth, "depth", 2, "BFS depth from --issue (ignored when --issue=0)")
	cmd.Flags().StringVar(&label, "label", "", "Filter nodes by label")
	cmd.Flags().StringVar(&milestone, "milestone", "", "Filter nodes by milestone title")
	cmd.Flags().BoolVar(&noTimeline, "no-timeline", false, "Skip cross-reference timeline queries")
	cmd.Flags().BoolVar(&noSubIssues, "no-sub-issues", false, "Skip sub-issue queries")
	cmd.Flags().StringVar(&jqFilter, "jq", "", "jq filter applied to JSON output")
	cmd.Flags().StringVar(&format, "format", "list", "Output format: list|tree|dot|json")

	return cmd
}

// ─── Graph builder ────────────────────────────────────────────────────────────

// buildGraph fetches all issues/PRs and constructs the relationship graph.
func buildGraph(
	gql *ghapi.GraphQLClient,
	owner, repo, labelFilter, milestoneFilter string,
	noTimeline, noSubIssues bool,
) (*model.Graph, error) {
	g := model.NewGraph()

	// Fetch all issues and PRs via pagination
	cursor := ""
	for {
		page, err := graphql.QueryIssuesPage(gql, owner, repo, cursor)
		if err != nil {
			return nil, fmt.Errorf("fetching issues page: %w", err)
		}

		for _, n := range page.Issues {
			if !matchesFilter(n.Labels, n.Milestone, labelFilter, milestoneFilter) {
				continue
			}
			g.AddNode(&model.Node{
				ID:     n.ID,
				Number: n.Number,
				Kind:   model.NodeKindIssue,
				State:  n.State,
				Title:  n.Title,
				URL:    n.URL,
				Labels: n.Labels,
			})
		}

		for _, n := range page.PRs {
			if !matchesFilter(n.Labels, n.Milestone, labelFilter, milestoneFilter) {
				continue
			}
			g.AddNode(&model.Node{
				ID:      n.ID,
				Number:  n.Number,
				Kind:    model.NodeKindPR,
				State:   n.State,
				Title:   n.Title,
				URL:     n.URL,
				Labels:  n.Labels,
				IsDraft: n.IsDraft,
			})
		}

		if !page.HasNextPage {
			break
		}
		cursor = page.Cursor
	}

	// Sub-issue edges
	if !noSubIssues {
		if err := addSubIssueEdges(gql, owner, repo, g); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "warning: sub-issue query failed: %v\n", err)
		}
	}

	// Cross-reference edges
	if !noTimeline {
		if err := addTimelineEdges(gql, owner, repo, g); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "warning: timeline query failed: %v\n", err)
		}
	}

	return g, nil
}

func addSubIssueEdges(gql *ghapi.GraphQLClient, owner, repo string, g *model.Graph) error {
	for _, n := range g.Nodes {
		if n.Kind != model.NodeKindIssue {
			continue
		}
		result, err := graphql.QuerySubIssues(gql, owner, repo, n.Number)
		if err != nil {
			return err
		}
		for _, child := range result.Children {
			childNode, ok := g.Nodes[child.ID]
			if !ok {
				childNode = &model.Node{
					ID:     child.ID,
					Number: child.Number,
					Kind:   model.NodeKindIssue,
					State:  child.State,
					Title:  child.Title,
					URL:    child.URL,
					Labels: child.Labels,
				}
				g.AddNode(childNode)
			}
			g.AddEdge(model.Edge{
				Source: n.ID,
				Target: childNode.ID,
				Type:   model.EdgeSubIssue,
			})
		}
	}
	return nil
}

func addTimelineEdges(gql *ghapi.GraphQLClient, owner, repo string, g *model.Graph) error {
	for _, n := range g.Nodes {
		items, err := graphql.QueryTimeline(gql, owner, repo, n.Number)
		if err != nil {
			return err
		}
		for _, item := range items {
			srcNode := findNodeByNumber(g, item.SourceNumber)
			tgtNode := findNodeByNumber(g, item.TargetNumber)
			if srcNode == nil || tgtNode == nil {
				continue
			}
			edgeType := model.EdgeCrossRef
			if item.ItemType == "ConnectedEvent" {
				edgeType = model.EdgeCloses
			}
			g.AddEdge(model.Edge{
				Source: srcNode.ID,
				Target: tgtNode.ID,
				Type:   edgeType,
			})
		}
	}
	return nil
}

// ─── Output formatters ────────────────────────────────────────────────────────

// renderGraph writes the graph to w in the requested format.
func renderGraph(w io.Writer, g *model.Graph, format, jqFilter string) error {
	switch format {
	case "dot":
		_, err := fmt.Fprint(w, formatDot(g))
		return err
	case "tree":
		_, err := fmt.Fprint(w, formatTree(g))
		return err
	case "json":
		out, err := formatJSON(g)
		if err != nil {
			return err
		}
		if jqFilter != "" {
			return applyJQ(w, out, jqFilter)
		}
		_, err = fmt.Fprintln(w, out)
		return err
	default: // "list" and anything else
		_, err := fmt.Fprint(w, formatList(g))
		return err
	}
}

// formatList returns one line per node: "#N [kind] [state] title"
func formatList(g *model.Graph) string {
	nodes := sortedNodes(g)
	var sb strings.Builder
	for _, n := range nodes {
		fmt.Fprintf(&sb, "#%d [%s] [%s] %s\n", n.Number, n.Kind, n.State, n.Title)
	}
	return sb.String()
}

// formatTree returns an indented tree representation using edge relationships.
func formatTree(g *model.Graph) string {
	// Find nodes that have incoming edges (have a parent)
	hasParent := make(map[string]bool)
	for _, e := range g.Edges {
		hasParent[e.Target] = true
	}

	// Build children map
	children := make(map[string][]string)
	for _, e := range g.Edges {
		children[e.Source] = append(children[e.Source], e.Target)
	}

	var sb strings.Builder

	// Collect root nodes (sorted for determinism)
	var roots []string
	for id := range g.Nodes {
		if !hasParent[id] {
			roots = append(roots, id)
		}
	}
	sort.Slice(roots, func(i, j int) bool {
		ni := g.Nodes[roots[i]]
		nj := g.Nodes[roots[j]]
		if ni == nil || nj == nil {
			return false
		}
		return ni.Number < nj.Number
	})

	visited := make(map[string]bool)
	var printNode func(id string, indent int)
	printNode = func(id string, indent int) {
		if visited[id] {
			return
		}
		visited[id] = true
		n := g.Nodes[id]
		if n == nil {
			return
		}
		prefix := strings.Repeat("  ", indent)
		fmt.Fprintf(&sb, "%s#%d [%s] [%s] %s\n", prefix, n.Number, n.Kind, n.State, n.Title)
		// Sort children for determinism
		ch := append([]string{}, children[id]...)
		sort.Slice(ch, func(i, j int) bool {
			ni := g.Nodes[ch[i]]
			nj := g.Nodes[ch[j]]
			if ni == nil || nj == nil {
				return false
			}
			return ni.Number < nj.Number
		})
		for _, childID := range ch {
			printNode(childID, indent+1)
		}
	}

	for _, rootID := range roots {
		printNode(rootID, 0)
	}

	// Print any unvisited nodes (disconnected nodes) at top level
	remaining := sortedNodes(g)
	for _, n := range remaining {
		if !visited[n.ID] {
			printNode(n.ID, 0)
		}
	}

	return sb.String()
}

// formatDot returns a Graphviz DOT digraph string.
func formatDot(g *model.Graph) string {
	var sb strings.Builder
	sb.WriteString("digraph G {\n")
	sb.WriteString("  node [shape=box];\n")

	// Nodes (sorted for determinism)
	nodes := sortedNodes(g)
	for _, n := range nodes {
		label := fmt.Sprintf("#%d %s", n.Number, n.Title)
		label = strings.ReplaceAll(label, `"`, `\"`)
		fmt.Fprintf(&sb, "  %q [label=%q];\n", n.ID, label)
	}

	// Edges
	for _, e := range g.Edges {
		fmt.Fprintf(&sb, "  %q -> %q [label=%q];\n", e.Source, e.Target, string(e.Type))
	}

	sb.WriteString("}")
	return sb.String()
}

// jsonGraph is the JSON representation of a graph.
type jsonGraph struct {
	Nodes []jsonNode `json:"nodes"`
	Edges []jsonEdge `json:"edges"`
	Stats jsonStats  `json:"stats"`
}

type jsonNode struct {
	ID      string         `json:"id"`
	Number  int            `json:"number"`
	Kind    model.NodeKind `json:"kind"`
	State   string         `json:"state"`
	Title   string         `json:"title"`
	URL     string         `json:"url"`
	Labels  []string       `json:"labels"`
	IsDraft bool           `json:"isDraft"`
}

type jsonEdge struct {
	Source string         `json:"source"`
	Target string         `json:"target"`
	Type   model.EdgeType `json:"type"`
}

type jsonStats struct {
	Total  int `json:"total"`
	Open   int `json:"open"`
	Closed int `json:"closed"`
	PRs    int `json:"prs"`
}

// formatJSON marshals the graph to a pretty-printed JSON string.
func formatJSON(g *model.Graph) (string, error) {
	nodes := sortedNodes(g)
	jNodes := make([]jsonNode, 0, len(nodes))
	for _, n := range nodes {
		labels := n.Labels
		if labels == nil {
			labels = []string{}
		}
		jNodes = append(jNodes, jsonNode{
			ID:      n.ID,
			Number:  n.Number,
			Kind:    n.Kind,
			State:   n.State,
			Title:   n.Title,
			URL:     n.URL,
			Labels:  labels,
			IsDraft: n.IsDraft,
		})
	}

	jEdges := make([]jsonEdge, 0, len(g.Edges))
	for _, e := range g.Edges {
		jEdges = append(jEdges, jsonEdge{
			Source: e.Source,
			Target: e.Target,
			Type:   e.Type,
		})
	}

	s := g.Stats()
	jg := jsonGraph{
		Nodes: jNodes,
		Edges: jEdges,
		Stats: jsonStats{
			Total:  s.Total,
			Open:   s.Open,
			Closed: s.Closed,
			PRs:    s.PRs,
		},
	}

	b, err := json.MarshalIndent(jg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// applyJQ applies a jq filter to JSON text and writes the result to w.
func applyJQ(w io.Writer, jsonText, filter string) error {
	q, err := gojq.Parse(filter)
	if err != nil {
		return fmt.Errorf("jq filter parse error: %w", err)
	}

	var input interface{}
	if err := json.Unmarshal([]byte(jsonText), &input); err != nil {
		return fmt.Errorf("jq: failed to parse JSON: %w", err)
	}

	iter := q.Run(input)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return fmt.Errorf("jq evaluation error: %w", err)
		}
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(b))
		if err != nil {
			return err
		}
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func sortedNodes(g *model.Graph) []*model.Node {
	nodes := make([]*model.Node, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Number < nodes[j].Number
	})
	return nodes
}

func findNodeByNumber(g *model.Graph, number int) *model.Node {
	for _, n := range g.Nodes {
		if n.Number == number {
			return n
		}
	}
	return nil
}

func findNodeIDByNumber(g *model.Graph, number int) string {
	n := findNodeByNumber(g, number)
	if n == nil {
		return ""
	}
	return n.ID
}

func matchesFilter(labels []string, nodeMilestone, labelFilter, milestoneFilter string) bool {
	if labelFilter != "" {
		found := false
		for _, l := range labels {
			if l == labelFilter {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if milestoneFilter != "" && nodeMilestone != milestoneFilter {
		return false
	}
	return true
}
