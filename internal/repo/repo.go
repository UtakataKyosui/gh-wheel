// Package repo resolves the target GitHub repository for a command invocation.
package repo

import (
	"fmt"

	ghrepo "github.com/cli/go-gh/v2/pkg/repository"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
)

// Resolve returns the target repository.
//
// If flagRepo is non-empty (from -R/--repo), it is parsed as "owner/repo".
// Otherwise, the repository is detected from the current working directory's
// git remote via go-gh (which also honours the GH_REPO environment variable).
func Resolve(flagRepo string) (ghrepo.Repository, error) {
	if flagRepo != "" {
		r, err := ghrepo.Parse(flagRepo)
		if err != nil {
			return ghrepo.Repository{}, cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
				fmt.Errorf("invalid repository %q: %w", flagRepo, err))
		}
		return r, nil
	}
	r, err := ghrepo.Current()
	if err != nil {
		return ghrepo.Repository{}, cliexit.NewUsage(cliexit.ErrCodeUsageNoRepo,
			fmt.Errorf("could not detect repository from current directory; use -R owner/repo: %w", err))
	}
	return r, nil
}
