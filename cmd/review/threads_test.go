package review

import (
	"strings"
	"testing"
)

// TestFilterThreads verifies that resolved and outdated threads are excluded.
func TestFilterThreads(t *testing.T) {
	threads := []reviewThread{
		{ID: "t1", Path: "foo.go", Line: 10, IsResolved: false, IsOutdated: false},
		{ID: "t2", Path: "bar.go", Line: 20, IsResolved: true, IsOutdated: false},
		{ID: "t3", Path: "baz.go", Line: 30, IsResolved: false, IsOutdated: true},
		{ID: "t4", Path: "qux.go", Line: 40, IsResolved: true, IsOutdated: true},
	}

	got := filterThreads(threads)

	if len(got) != 1 {
		t.Fatalf("expected 1 unresolved non-outdated thread, got %d", len(got))
	}
	if got[0].ID != "t1" {
		t.Errorf("expected thread ID %q, got %q", "t1", got[0].ID)
	}
}

// TestFilterThreads_Empty verifies empty input returns empty slice.
func TestFilterThreads_Empty(t *testing.T) {
	got := filterThreads(nil)
	if len(got) != 0 {
		t.Errorf("expected 0 threads, got %d", len(got))
	}
}

// TestTruncateComment verifies that comments are truncated to 60 chars.
func TestTruncateComment(t *testing.T) {
	long := strings.Repeat("a", 80)
	got := truncateComment(long, 60)
	if len(got) != 63 { // 60 chars + "..."
		t.Errorf("expected length 63, got %d: %q", len(got), got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected suffix %q, got %q", "...", got)
	}
}

// TestTruncateComment_Short verifies that short comments are not truncated.
func TestTruncateComment_Short(t *testing.T) {
	short := "hello"
	got := truncateComment(short, 60)
	if got != short {
		t.Errorf("expected %q, got %q", short, got)
	}
}

// TestTruncateComment_Exact verifies that a comment of exactly limit chars is not truncated.
func TestTruncateComment_Exact(t *testing.T) {
	exact := strings.Repeat("b", 60)
	got := truncateComment(exact, 60)
	if got != exact {
		t.Errorf("expected no truncation for exact-limit string, got %q", got)
	}
}

// TestPrintThreadsTable_NoThreads verifies empty-state message.
func TestPrintThreadsTable_NoThreads(t *testing.T) {
	var buf strings.Builder
	printThreadsTable(&buf, []reviewThread{})
	// Should produce no table rows
	if strings.Contains(buf.String(), "|") {
		t.Errorf("expected no table output for empty threads, got: %q", buf.String())
	}
}

// TestPrintThreadsTable_WithThread verifies table output contains expected fields.
func TestPrintThreadsTable_WithThread(t *testing.T) {
	c := threadComment{ID: "c1", Body: "This needs a unit test"}
	c.Author.Login = "reviewer1"

	t1 := reviewThread{ID: "PRRT_abc123", Path: "pkg/foo.go", Line: 42}
	t1.Comments.Nodes = []threadComment{c}

	threads := []reviewThread{t1}

	var buf strings.Builder
	printThreadsTable(&buf, threads)
	output := buf.String()

	if !strings.Contains(output, "pkg/foo.go") {
		t.Errorf("expected path in output, got: %q", output)
	}
	if !strings.Contains(output, "reviewer1") {
		t.Errorf("expected author in output, got: %q", output)
	}
	if !strings.Contains(output, "This needs a unit test") {
		t.Errorf("expected comment body in output, got: %q", output)
	}
}
