package repo_test

import (
	"testing"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/repo"
)

func TestResolve_ExplicitFlag_Valid(t *testing.T) {
	r, err := repo.Resolve("owner/repo")
	if err != nil {
		t.Fatalf("Resolve(owner/repo) error: %v", err)
	}
	// ghrepo.Repository has exported string fields Owner and Name.
	if r.Owner != "owner" {
		t.Errorf("owner = %q, want %q", r.Owner, "owner")
	}
	if r.Name != "repo" {
		t.Errorf("name = %q, want %q", r.Name, "repo")
	}
}

func TestResolve_ExplicitFlag_Invalid(t *testing.T) {
	_, err := repo.Resolve("not-a-valid-repo-format-with-no-slash-what-a-mess!!!")
	if err == nil {
		t.Fatal("expected error for invalid repo format, got nil")
	}
	if code := cliexit.ExitCodeOf(err); code != cliexit.CodeUsage {
		t.Errorf("exit code = %d, want CodeUsage (%d)", code, cliexit.CodeUsage)
	}
}
