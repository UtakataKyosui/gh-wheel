package ghclient_test

import (
	"testing"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
)

// TestNew_InvalidRepoFlag_NoSlash verifies that a value without a slash (which
// go-gh's repository.Parse cannot interpret as owner/repo) returns an error.
func TestNew_InvalidRepoFlag_NoSlash(t *testing.T) {
	_, err := ghclient.New("noslashreponame")
	if err == nil {
		t.Fatal("expected error for repo flag without slash, got nil")
	}
	code := cliexit.ExitCodeOf(err)
	if code != cliexit.CodeUsage && code != cliexit.CodeAuth {
		t.Errorf("exit code = %d, want CodeUsage (%d) or CodeAuth (%d)",
			code, cliexit.CodeUsage, cliexit.CodeAuth)
	}
}

// TestNew_ValidRepoFlag_AccessorsOrAuthError verifies that when a valid
// owner/repo is provided and auth succeeds, Owner/Name return the right values.
// In CI without credentials this will return CodeAuth, which is also acceptable.
func TestNew_ValidRepoFlag_AccessorsOrAuthError(t *testing.T) {
	c, err := ghclient.New("owner/repo")
	if err != nil {
		code := cliexit.ExitCodeOf(err)
		if code != cliexit.CodeAuth {
			t.Fatalf("unexpected error (code %d): %v", code, err)
		}
		t.Logf("skipping accessor check: no gh credentials in this environment (%v)", err)
		return
	}
	if c.Owner() != "owner" {
		t.Errorf("Owner() = %q, want %q", c.Owner(), "owner")
	}
	if c.Name() != "repo" {
		t.Errorf("Name() = %q, want %q", c.Name(), "repo")
	}
}
