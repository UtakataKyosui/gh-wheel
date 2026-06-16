// Package graph provides the in-memory graph model for GitHub Issue/PR relationship
// graphs, including BFS traversal and statistics computation.
package graph

// NodeKind distinguishes Issues from Pull Requests.
type NodeKind string

const (
	NodeKindIssue NodeKind = "issue"
	NodeKindPR    NodeKind = "pull_request"
)

// EdgeType describes the relationship between two nodes.
type EdgeType string

const (
	EdgeCloses   EdgeType = "closes"
	EdgeCrossRef EdgeType = "cross_ref"
	EdgeSubIssue EdgeType = "sub_issue"
)

// Node represents a single GitHub Issue or Pull Request.
type Node struct {
	ID      string
	Number  int
	Kind    NodeKind
	State   string
	Title   string
	URL     string
	Labels  []string
	IsDraft bool
}

// Edge represents a directed relationship between two nodes.
type Edge struct {
	Source string
	Target string
	Type   EdgeType
}

// Graph holds the full set of nodes and edges for an issue/PR relationship graph.
type Graph struct {
	Nodes map[string]*Node
	Edges []Edge
}

// NewGraph returns a new, empty Graph with initialized maps/slices.
func NewGraph() *Graph {
	return &Graph{
		Nodes: make(map[string]*Node),
		Edges: make([]Edge, 0),
	}
}

// AddNode inserts a node into the graph, keyed by its ID.
// If a node with the same ID already exists it is overwritten.
func (g *Graph) AddNode(n *Node) {
	g.Nodes[n.ID] = n
}

// AddEdge appends a directed edge to the graph.
func (g *Graph) AddEdge(e Edge) {
	g.Edges = append(g.Edges, e)
}

// GraphStats holds aggregate counts about a graph.
type GraphStats struct {
	Total  int
	Open   int
	Closed int
	PRs    int
}

// Stats computes summary statistics for the graph.
// A node is counted as "open" when its State is "OPEN".
// MERGED and CLOSED states both contribute to Closed.
func (g *Graph) Stats() GraphStats {
	s := GraphStats{Total: len(g.Nodes)}
	for _, n := range g.Nodes {
		if n.Kind == NodeKindPR {
			s.PRs++
		}
		switch n.State {
		case "OPEN":
			s.Open++
		default:
			s.Closed++
		}
	}
	return s
}

// BFS returns the subgraph reachable from startID within the given depth.
// Edges are traversed in both directions (undirected BFS) so that callers
// discover both parents and children of a start node.
// Depth 0 returns only the start node itself.
// If startID does not exist in the graph an empty graph is returned.
func (g *Graph) BFS(startID string, depth int) *Graph {
	sub := NewGraph()

	start, ok := g.Nodes[startID]
	if !ok {
		return sub
	}

	// adjacency: node ID -> set of neighbor IDs (both directions)
	adj := buildAdjacency(g)

	// BFS queue of (id, currentDepth) pairs
	type entry struct {
		id    string
		depth int
	}

	visited := make(map[string]bool)
	queue := []entry{{id: startID, depth: 0}}
	visited[startID] = true
	sub.AddNode(start)

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if curr.depth >= depth {
			continue
		}

		for neighborID := range adj[curr.id] {
			if visited[neighborID] {
				continue
			}
			visited[neighborID] = true
			if n, exists := g.Nodes[neighborID]; exists {
				sub.AddNode(n)
			}
			queue = append(queue, entry{id: neighborID, depth: curr.depth + 1})
		}
	}

	// Copy only edges where both endpoints are in the subgraph
	for _, e := range g.Edges {
		_, srcOk := sub.Nodes[e.Source]
		_, tgtOk := sub.Nodes[e.Target]
		if srcOk && tgtOk {
			sub.AddEdge(e)
		}
	}

	return sub
}

// buildAdjacency constructs an undirected adjacency map from the graph's edges.
func buildAdjacency(g *Graph) map[string]map[string]struct{} {
	adj := make(map[string]map[string]struct{})
	for _, e := range g.Edges {
		if adj[e.Source] == nil {
			adj[e.Source] = make(map[string]struct{})
		}
		if adj[e.Target] == nil {
			adj[e.Target] = make(map[string]struct{})
		}
		adj[e.Source][e.Target] = struct{}{}
		adj[e.Target][e.Source] = struct{}{} // bidirectional
	}
	return adj
}
