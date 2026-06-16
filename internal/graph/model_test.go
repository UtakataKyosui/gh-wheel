package graph

import (
	"testing"
)

// TestGraph_AddNode verifies that nodes are added to the graph correctly.
func TestGraph_AddNode(t *testing.T) {
	g := NewGraph()

	n := &Node{
		ID:     "issue_1",
		Number: 1,
		Kind:   NodeKindIssue,
		State:  "OPEN",
		Title:  "First Issue",
		URL:    "https://github.com/owner/repo/issues/1",
		Labels: []string{"bug"},
	}
	g.AddNode(n)

	if len(g.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(g.Nodes))
	}
	got, ok := g.Nodes["issue_1"]
	if !ok {
		t.Fatal("node with ID 'issue_1' not found in graph")
	}
	if got.Number != 1 {
		t.Errorf("Number: want 1, got %d", got.Number)
	}
	if got.Kind != NodeKindIssue {
		t.Errorf("Kind: want %q, got %q", NodeKindIssue, got.Kind)
	}
	if got.Title != "First Issue" {
		t.Errorf("Title: want 'First Issue', got %q", got.Title)
	}
}

// TestGraph_AddEdge verifies that edges are added to the graph correctly.
func TestGraph_AddEdge(t *testing.T) {
	g := NewGraph()

	n1 := &Node{ID: "pr_5", Number: 5, Kind: NodeKindPR}
	n2 := &Node{ID: "issue_1", Number: 1, Kind: NodeKindIssue}
	g.AddNode(n1)
	g.AddNode(n2)

	e := Edge{Source: "pr_5", Target: "issue_1", Type: EdgeCloses}
	g.AddEdge(e)

	if len(g.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(g.Edges))
	}
	if g.Edges[0].Source != "pr_5" {
		t.Errorf("Edge Source: want 'pr_5', got %q", g.Edges[0].Source)
	}
	if g.Edges[0].Target != "issue_1" {
		t.Errorf("Edge Target: want 'issue_1', got %q", g.Edges[0].Target)
	}
	if g.Edges[0].Type != EdgeCloses {
		t.Errorf("Edge Type: want %q, got %q", EdgeCloses, g.Edges[0].Type)
	}
}

// TestGraph_Stats verifies that graph statistics are computed correctly.
func TestGraph_Stats(t *testing.T) {
	g := NewGraph()

	g.AddNode(&Node{ID: "i1", Number: 1, Kind: NodeKindIssue, State: "OPEN"})
	g.AddNode(&Node{ID: "i2", Number: 2, Kind: NodeKindIssue, State: "CLOSED"})
	g.AddNode(&Node{ID: "i3", Number: 3, Kind: NodeKindIssue, State: "OPEN"})
	g.AddNode(&Node{ID: "pr4", Number: 4, Kind: NodeKindPR, State: "OPEN"})
	g.AddNode(&Node{ID: "pr5", Number: 5, Kind: NodeKindPR, State: "MERGED"})

	stats := g.Stats()

	if stats.Total != 5 {
		t.Errorf("Total: want 5, got %d", stats.Total)
	}
	if stats.Open != 3 {
		t.Errorf("Open: want 3, got %d", stats.Open)
	}
	if stats.Closed != 2 {
		t.Errorf("Closed: want 2, got %d", stats.Closed)
	}
	if stats.PRs != 2 {
		t.Errorf("PRs: want 2, got %d", stats.PRs)
	}
}

// TestGraph_BFS_Depth verifies BFS returns correct subgraph within depth limit.
func TestGraph_BFS_Depth(t *testing.T) {
	// Build a chain: root -> child1 -> grandchild -> great_grandchild
	g := NewGraph()
	g.AddNode(&Node{ID: "root", Number: 1, Kind: NodeKindIssue})
	g.AddNode(&Node{ID: "child1", Number: 2, Kind: NodeKindIssue})
	g.AddNode(&Node{ID: "grandchild", Number: 3, Kind: NodeKindIssue})
	g.AddNode(&Node{ID: "great_grandchild", Number: 4, Kind: NodeKindIssue})

	g.AddEdge(Edge{Source: "root", Target: "child1", Type: EdgeCloses})
	g.AddEdge(Edge{Source: "child1", Target: "grandchild", Type: EdgeCloses})
	g.AddEdge(Edge{Source: "grandchild", Target: "great_grandchild", Type: EdgeCloses})

	t.Run("depth=1 returns root and direct neighbors", func(t *testing.T) {
		sub := g.BFS("root", 1)
		if len(sub.Nodes) != 2 {
			t.Errorf("BFS depth=1: want 2 nodes (root+child1), got %d", len(sub.Nodes))
		}
		if _, ok := sub.Nodes["root"]; !ok {
			t.Error("BFS depth=1: root should be in subgraph")
		}
		if _, ok := sub.Nodes["child1"]; !ok {
			t.Error("BFS depth=1: child1 should be in subgraph")
		}
		if _, ok := sub.Nodes["grandchild"]; ok {
			t.Error("BFS depth=1: grandchild should NOT be in subgraph")
		}
	})

	t.Run("depth=2 returns root, child, grandchild", func(t *testing.T) {
		sub := g.BFS("root", 2)
		if len(sub.Nodes) != 3 {
			t.Errorf("BFS depth=2: want 3 nodes, got %d", len(sub.Nodes))
		}
		if _, ok := sub.Nodes["grandchild"]; !ok {
			t.Error("BFS depth=2: grandchild should be in subgraph")
		}
		if _, ok := sub.Nodes["great_grandchild"]; ok {
			t.Error("BFS depth=2: great_grandchild should NOT be in subgraph")
		}
	})

	t.Run("depth=0 returns only root", func(t *testing.T) {
		sub := g.BFS("root", 0)
		if len(sub.Nodes) != 1 {
			t.Errorf("BFS depth=0: want 1 node, got %d", len(sub.Nodes))
		}
		if _, ok := sub.Nodes["root"]; !ok {
			t.Error("BFS depth=0: root should be in subgraph")
		}
	})

	t.Run("unknown startID returns empty graph", func(t *testing.T) {
		sub := g.BFS("nonexistent", 2)
		if len(sub.Nodes) != 0 {
			t.Errorf("BFS unknown id: want 0 nodes, got %d", len(sub.Nodes))
		}
	})
}

// TestGraph_BFS_Bidirectional verifies BFS traverses edges in both directions.
func TestGraph_BFS_Bidirectional(t *testing.T) {
	// Parent <-> child connection: edge direction is parent -> child but BFS should
	// also discover parent when starting from child.
	g := NewGraph()
	g.AddNode(&Node{ID: "parent", Number: 10, Kind: NodeKindIssue})
	g.AddNode(&Node{ID: "child", Number: 11, Kind: NodeKindIssue})
	g.AddEdge(Edge{Source: "parent", Target: "child", Type: EdgeSubIssue})

	sub := g.BFS("child", 1)
	if _, ok := sub.Nodes["parent"]; !ok {
		t.Error("BFS from child should reach parent via reverse edge traversal")
	}
}

// TestNodeKindConstants verifies NodeKind constants are correct.
func TestNodeKindConstants(t *testing.T) {
	if NodeKindIssue != "issue" {
		t.Errorf("NodeKindIssue: want 'issue', got %q", NodeKindIssue)
	}
	if NodeKindPR != "pull_request" {
		t.Errorf("NodeKindPR: want 'pull_request', got %q", NodeKindPR)
	}
}

// TestEdgeTypeConstants verifies EdgeType constants are correct.
func TestEdgeTypeConstants(t *testing.T) {
	if EdgeCloses != "closes" {
		t.Errorf("EdgeCloses: want 'closes', got %q", EdgeCloses)
	}
	if EdgeCrossRef != "cross_ref" {
		t.Errorf("EdgeCrossRef: want 'cross_ref', got %q", EdgeCrossRef)
	}
	if EdgeSubIssue != "sub_issue" {
		t.Errorf("EdgeSubIssue: want 'sub_issue', got %q", EdgeSubIssue)
	}
}

// TestNewGraph verifies NewGraph returns an initialized graph.
func TestNewGraph(t *testing.T) {
	g := NewGraph()
	if g == nil {
		t.Fatal("NewGraph() returned nil")
	}
	if g.Nodes == nil {
		t.Error("Nodes map should be initialized")
	}
	if g.Edges == nil {
		t.Error("Edges slice should be initialized")
	}
	if len(g.Nodes) != 0 {
		t.Errorf("New graph should have 0 nodes, got %d", len(g.Nodes))
	}
}
