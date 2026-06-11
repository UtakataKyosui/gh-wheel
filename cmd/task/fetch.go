package task

import (
	"fmt"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
)

// searchPage is the GitHub Search API response envelope.
type searchPage struct {
	TotalCount int          `json:"total_count"`
	Items      []searchItem `json:"items"`
}

// searchItem maps fields from a GitHub Search API item.
// Note: the Search API does not return PR head details (head.ref); that
// requires a separate GET /repos/{owner}/{repo}/pulls/{number} call.
type searchItem struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	HTMLURL   string    `json:"html_url"`
	State     string    `json:"state"`
	Draft     bool      `json:"draft"`
	Body      string    `json:"body"`
	UpdatedAt time.Time `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	PullRequest *struct{} `json:"pull_request"`
}

// fetchOpts controls what the fetch function retrieves.
type fetchOpts struct {
	State         string
	AuthorOnly    bool
	ReviewOnly    bool
	IncludeDrafts bool
	WithIssues    bool
	IssuesOnly    bool
	WithReviews   bool
}

func fetch(c *ghclient.Client, login string, opts fetchOpts) (*TaskResult, error) {
	var (
		wg        sync.WaitGroup
		authorRes []searchItem
		reviewRes []searchItem
		issueRes  []searchItem
		errs      = make([]error, 3)
	)

	repo := fmt.Sprintf("%s/%s", c.Owner(), c.Name())
	stateQ := stateQuery(opts.State)

	if !opts.IssuesOnly {
		if !opts.ReviewOnly {
			wg.Add(1)
			go func() {
				defer wg.Done()
				q := fmt.Sprintf("is:pr repo:%s author:@me%s", repo, stateQ)
				authorRes, errs[0] = searchItems(c, q)
			}()
		}
		if !opts.AuthorOnly {
			wg.Add(1)
			go func() {
				defer wg.Done()
				q := fmt.Sprintf("is:pr repo:%s review-requested:@me%s", repo, stateQ)
				reviewRes, errs[1] = searchItems(c, q)
			}()
		}
	}

	if opts.WithIssues || opts.IssuesOnly {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q := fmt.Sprintf("is:issue repo:%s assignee:@me%s", repo, stateQ)
			issueRes, errs[2] = searchItems(c, q)
		}()
	}

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	prs := mergePRs(authorRes, reviewRes, opts.IncludeDrafts)

	if opts.WithReviews && len(prs) > 0 {
		if err := attachDetails(c, prs); err != nil {
			return nil, err
		}
	}

	return &TaskResult{
		Repository: repo,
		User:       login,
		FetchedAt:  time.Now().UTC(),
		PRs:        prs,
		Issues:     toIssues(issueRes),
	}, nil
}

func searchItems(c *ghclient.Client, q string) ([]searchItem, error) {
	// url.QueryEscape encodes spaces as '+', which is the correct
	// application/x-www-form-urlencoded representation for the q parameter.
	v := url.Values{"per_page": {"100"}, "q": {q}}
	endpoint := "search/issues?" + v.Encode()
	var page searchPage
	if err := c.Get(endpoint, &page); err != nil {
		return nil, err
	}
	return page.Items, nil
}

func stateQuery(state string) string {
	switch state {
	case "closed":
		return " is:closed"
	case "all":
		return ""
	default:
		return " is:open"
	}
}

// mergePRs deduplicates author and review-requested lists and assigns categories.
func mergePRs(authorItems, reviewItems []searchItem, includeDrafts bool) []PR {
	type entry struct {
		item   searchItem
		isAuth bool
		isRev  bool
	}

	byNum := make(map[int]*entry, len(authorItems)+len(reviewItems))

	for _, it := range authorItems {
		it := it
		byNum[it.Number] = &entry{item: it, isAuth: true}
	}
	for _, it := range reviewItems {
		it := it
		if e, ok := byNum[it.Number]; ok {
			e.isRev = true
		} else {
			byNum[it.Number] = &entry{item: it, isRev: true}
		}
	}

	prs := make([]PR, 0, len(byNum))
	for _, e := range byNum {
		if !includeDrafts && e.item.Draft {
			continue
		}

		var cats []string
		if e.isAuth {
			cats = append(cats, "author")
		}
		if e.isRev {
			cats = append(cats, "review-requested")
		}

		labels := make([]string, len(e.item.Labels))
		for i, l := range e.item.Labels {
			labels[i] = l.Name
		}

		prs = append(prs, PR{
			Number:         e.item.Number,
			Title:          e.item.Title,
			URL:            e.item.HTMLURL,
			Author:         e.item.User.Login,
			State:          e.item.State,
			IsDraft:        e.item.Draft,
			UpdatedAt:      e.item.UpdatedAt,
			Labels:         labels,
			Categories:     cats,
			Body:           e.item.Body,
			HeadRef:        "", // populated by attachDetails when --with-reviews is set
			Reviews:        []Review{},
			ReviewComments: []ReviewComment{},
		})
	}

	sort.Slice(prs, func(i, j int) bool {
		return prs[i].UpdatedAt.After(prs[j].UpdatedAt)
	})
	return prs
}

func toIssues(items []searchItem) []Issue {
	if len(items) == 0 {
		return []Issue{}
	}
	issues := make([]Issue, len(items))
	for i, it := range items {
		labels := make([]string, len(it.Labels))
		for j, l := range it.Labels {
			labels[j] = l.Name
		}
		issues[i] = Issue{
			Number:    it.Number,
			Title:     it.Title,
			URL:       it.HTMLURL,
			Author:    it.User.Login,
			State:     it.State,
			UpdatedAt: it.UpdatedAt,
			Labels:    labels,
			Body:      it.Body,
		}
	}
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].UpdatedAt.After(issues[j].UpdatedAt)
	})
	return issues
}

// apiPRDetail maps the PR-specific fields from GET /repos/{owner}/{repo}/pulls/{number}.
type apiPRDetail struct {
	Head struct {
		Ref string `json:"ref"`
	} `json:"head"`
}

// apiReview maps a single review from GET /repos/{owner}/{repo}/pulls/{number}/reviews.
type apiReview struct {
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	State       string    `json:"state"`
	Body        string    `json:"body"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// apiReviewComment maps an inline review comment from
// GET /repos/{owner}/{repo}/pulls/{number}/comments.
type apiReviewComment struct {
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	Body      string    `json:"body"`
	Path      string    `json:"path"`
	UpdatedAt time.Time `json:"updated_at"`
}

// attachDetails fetches PR details, reviews, and review comments for each PR in
// parallel, bounded to 10 concurrent requests to avoid GitHub secondary rate limits.
func attachDetails(c *ghclient.Client, prs []PR) error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	sem := make(chan struct{}, 10)

	for i := range prs {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			num := prs[i].Number

			// Fetch PR details for HeadRef.
			var detail apiPRDetail
			if err := c.RepoGet(fmt.Sprintf("pulls/%d", num), &detail); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}

			// Fetch reviews with per_page=100 to avoid silent truncation.
			var rawReviews []apiReview
			if err := c.RepoGet(fmt.Sprintf("pulls/%d/reviews?per_page=100", num), &rawReviews); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}

			// Fetch inline review comments with per_page=100.
			var rawComments []apiReviewComment
			if err := c.RepoGet(fmt.Sprintf("pulls/%d/comments?per_page=100", num), &rawComments); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}

			reviews := make([]Review, len(rawReviews))
			for j, r := range rawReviews {
				reviews[j] = Review{
					Author:      r.User.Login,
					State:       r.State,
					Body:        r.Body,
					SubmittedAt: r.SubmittedAt,
				}
			}

			comments := make([]ReviewComment, len(rawComments))
			for j, rc := range rawComments {
				comments[j] = ReviewComment{
					Author:    rc.User.Login,
					Body:      rc.Body,
					Path:      rc.Path,
					UpdatedAt: rc.UpdatedAt,
				}
			}

			mu.Lock()
			prs[i].HeadRef = detail.Head.Ref
			prs[i].Reviews = reviews
			prs[i].ReviewComments = comments
			mu.Unlock()
		}()
	}

	wg.Wait()

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
