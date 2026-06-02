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
	Head        *struct {
		Ref string `json:"ref"`
	} `json:"head"`
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
				q := fmt.Sprintf("is:pr+repo:%s+author:@me%s", repo, stateQ)
				authorRes, errs[0] = searchItems(c, q)
			}()
		}
		if !opts.AuthorOnly {
			wg.Add(1)
			go func() {
				defer wg.Done()
				q := fmt.Sprintf("is:pr+repo:%s+review-requested:@me%s", repo, stateQ)
				reviewRes, errs[1] = searchItems(c, q)
			}()
		}
	}

	if opts.WithIssues || opts.IssuesOnly {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q := fmt.Sprintf("is:issue+repo:%s+assignee:@me%s", repo, stateQ)
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
		if err := attachReviews(c, prs); err != nil {
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
	endpoint := fmt.Sprintf("search/issues?per_page=100&q=%s", url.QueryEscape(q))
	var page searchPage
	if err := c.Get(endpoint, &page); err != nil {
		return nil, err
	}
	return page.Items, nil
}

func stateQuery(state string) string {
	switch state {
	case "closed":
		return "+is:closed"
	case "all":
		return ""
	default:
		return "+is:open"
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

		headRef := ""
		if e.item.Head != nil {
			headRef = e.item.Head.Ref
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
			HeadRef:        headRef,
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

// apiReview maps a single review from the GitHub Reviews API.
type apiReview struct {
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	State       string    `json:"state"`
	Body        string    `json:"body"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// attachReviews fetches PR reviews in parallel and sets them on the PR slice.
func attachReviews(c *ghclient.Client, prs []PR) error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	for i := range prs {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			var reviews []apiReview
			path := fmt.Sprintf("pulls/%d/reviews", prs[i].Number)
			if err := c.RepoGet(path, &reviews); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}
			out := make([]Review, len(reviews))
			for j, r := range reviews {
				out[j] = Review{
					Author:      r.User.Login,
					State:       r.State,
					Body:        r.Body,
					SubmittedAt: r.SubmittedAt,
				}
			}
			mu.Lock()
			prs[i].Reviews = out
			mu.Unlock()
		}()
	}

	wg.Wait()

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
