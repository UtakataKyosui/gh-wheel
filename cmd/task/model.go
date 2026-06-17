package task

import "time"

// PR is a pull request entry in the task output.
type PR struct {
	Number     int       `json:"number"`
	Title      string    `json:"title"`
	URL        string    `json:"url"`
	Author     string    `json:"author"`
	State      string    `json:"state"`
	IsDraft    bool      `json:"isDraft"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Labels     []string  `json:"labels"`
	Categories []string  `json:"categories"` // "author" and/or "review-requested"
	Body       string    `json:"body"`
	HeadRef    string    `json:"headRef"`
	// Populated only when --with-reviews is set.
	Reviews        []Review        `json:"reviews"`
	ReviewComments []ReviewComment `json:"reviewComments"`
	LatestCommitAt time.Time       `json:"latestCommitAt"`
}

// Review is a single review on a PR.
type Review struct {
	Author      string    `json:"author"`
	UserType    string    `json:"userType"` // "User" or "Bot"
	State       string    `json:"state"`
	Body        string    `json:"body"`
	SubmittedAt time.Time `json:"submittedAt"`
}

// ReviewComment is an inline review comment on a PR.
type ReviewComment struct {
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	Path      string    `json:"path"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Issue is a GitHub issue entry in the task output.
type Issue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Author    string    `json:"author"`
	State     string    `json:"state"`
	UpdatedAt time.Time `json:"updatedAt"`
	Labels    []string  `json:"labels"`
	Body      string    `json:"body"`
}

// TaskResult is the top-level output for `gh wheel task`.
type TaskResult struct {
	SchemaVersion string    `json:"schema_version"`
	Kind          string    `json:"kind"`
	Repository    string    `json:"repository"`
	User          string    `json:"user"`
	FetchedAt     time.Time `json:"fetchedAt"`
	PRs           []PR      `json:"prs"`
	Issues        []Issue   `json:"issues"`
}
