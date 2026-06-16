// Package graphql provides GitHub GraphQL data-fetching helpers for gh-wheel.
// It exposes typed functions for querying issues/PRs, sub-issue hierarchies,
// and cross-reference timeline events.
package graphql

import (
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

// ─── Exported types ───────────────────────────────────────────────────────────

// Node represents a single GitHub issue or pull request.
type Node struct {
	ID        string
	Number    int
	Kind      string // "issue" or "pull_request"
	State     string
	Title     string
	URL       string
	Labels    []string
	Milestone string
	Assignees []string
	IsDraft   bool
}

// IssuesPage holds one page of issues and PRs fetched together.
type IssuesPage struct {
	Issues      []Node
	PRs         []Node
	HasNextPage bool
	Cursor      string
}

// SubIssueResult holds a parent issue and its direct sub-issues.
type SubIssueResult struct {
	Parent   *Node
	Children []Node
}

// TimelineItem represents a cross-reference or connected event on a timeline.
type TimelineItem struct {
	SourceNumber int
	TargetNumber int
	ItemType     string
}

// ─── Raw GraphQL response types ───────────────────────────────────────────────

type rawNode struct {
	ID      string `json:"id"`
	Number  int    `json:"number"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	State   string `json:"state"`
	IsDraft bool   `json:"isDraft"`
	Labels  struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	Milestone struct {
		Title string `json:"title"`
	} `json:"milestone"`
	Assignees struct {
		Nodes []struct {
			Login string `json:"login"`
		} `json:"nodes"`
	} `json:"assignees"`
	ClosingIssuesReferences struct {
		Nodes []struct {
			Number int `json:"number"`
		} `json:"nodes"`
	} `json:"closingIssuesReferences"`
}

type rawPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type rawIssuesPage struct {
	Repository struct {
		Issues struct {
			PageInfo rawPageInfo `json:"pageInfo"`
			Nodes    []rawNode   `json:"nodes"`
		} `json:"issues"`
		PRs struct {
			PageInfo rawPageInfo `json:"pageInfo"`
			Nodes    []rawNode   `json:"nodes"`
		} `json:"pullRequests"`
	} `json:"repository"`
}

type rawSubIssueResponse struct {
	Repository struct {
		Issue struct {
			rawNode
			Parent struct {
				rawNode
			} `json:"parent"`
			SubIssues struct {
				Nodes []rawNode `json:"nodes"`
			} `json:"subIssues"`
		} `json:"issue"`
	} `json:"repository"`
}

type rawTimelineSource struct {
	Typename string `json:"__typename"`
	Number   int    `json:"number"`
}

type rawTimelineSubject struct {
	Typename string `json:"__typename"`
	Number   int    `json:"number"`
}

type rawTimelineItem struct {
	Typename string             `json:"__typename"`
	Source   rawTimelineSource  `json:"source"`
	Subject  rawTimelineSubject `json:"subject"`
}

type rawTimelineResponse struct {
	Repository struct {
		IssueOrPullRequest struct {
			TimelineItems struct {
				Nodes []rawTimelineItem `json:"nodes"`
			} `json:"timelineItems"`
		} `json:"issueOrPullRequest"`
	} `json:"repository"`
}

// ─── Query functions ──────────────────────────────────────────────────────────

// QueryIssuesPage fetches up to 50 issues and 50 PRs (sorted by UPDATED_AT desc)
// using cursor-based pagination. Pass an empty cursor for the first page.
func QueryIssuesPage(gql *api.GraphQLClient, owner, repo, cursor string) (IssuesPage, error) {
	query := `
query($owner: String!, $repo: String!, $cursor: String) {
  repository(owner: $owner, name: $repo) {
    issues(first: 50, after: $cursor, orderBy: {field: UPDATED_AT, direction: DESC}) {
      pageInfo { hasNextPage endCursor }
      nodes {
        id number title url state
        labels(first: 20) { nodes { name } }
        milestone { title }
        assignees(first: 10) { nodes { login } }
      }
    }
    pullRequests(first: 50, after: $cursor, orderBy: {field: UPDATED_AT, direction: DESC}) {
      pageInfo { hasNextPage endCursor }
      nodes {
        id number title url state isDraft
        labels(first: 20) { nodes { name } }
        milestone { title }
        assignees(first: 10) { nodes { login } }
        closingIssuesReferences(first: 20) { nodes { number } }
      }
    }
  }
}`

	variables := map[string]interface{}{
		"owner":  owner,
		"repo":   repo,
		"cursor": nil,
	}
	if cursor != "" {
		variables["cursor"] = cursor
	}

	var raw rawIssuesPage
	if err := gql.Do(query, variables, &raw); err != nil {
		return IssuesPage{}, err
	}
	return convertIssuesPage(raw), nil
}

// QuerySubIssues fetches the parent and sub-issues for the given issue number.
// If the repository's GraphQL schema does not support subIssues (older GitHub
// Enterprise versions), the function returns an empty SubIssueResult and no error.
func QuerySubIssues(gql *api.GraphQLClient, owner, repo string, issueNum int) (SubIssueResult, error) {
	query := `
query($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    issue(number: $number) {
      id number title url state
      labels(first: 20) { nodes { name } }
      milestone { title }
      assignees(first: 10) { nodes { login } }
      parent {
        id number title url state
        labels(first: 20) { nodes { name } }
        milestone { title }
        assignees(first: 10) { nodes { login } }
      }
      subIssues(first: 50) {
        nodes {
          id number title url state
          labels(first: 20) { nodes { name } }
          milestone { title }
          assignees(first: 10) { nodes { login } }
        }
      }
    }
  }
}`

	variables := map[string]interface{}{
		"owner":  owner,
		"repo":   repo,
		"number": issueNum,
	}

	var raw rawSubIssueResponse
	if err := gql.Do(query, variables, &raw); err != nil {
		if isSubIssueSchemaError(err.Error()) {
			return SubIssueResult{}, nil
		}
		return SubIssueResult{}, err
	}
	return convertSubIssueResult(raw), nil
}

// QueryTimeline fetches cross-reference and connected timeline events for
// the given issue or PR number.
// If the schema does not support the requested item types, nil is returned
// without an error (graceful fallback).
func QueryTimeline(gql *api.GraphQLClient, owner, repo string, issueNum int) ([]TimelineItem, error) {
	query := `
query($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    issueOrPullRequest(number: $number) {
      ... on Issue {
        timelineItems(first: 100, itemTypes: [CROSS_REFERENCED_EVENT, CONNECTED_EVENT]) {
          nodes {
            __typename
            ... on CrossReferencedEvent {
              source {
                __typename
                ... on Issue { number }
                ... on PullRequest { number }
              }
            }
            ... on ConnectedEvent {
              subject {
                __typename
                ... on Issue { number }
                ... on PullRequest { number }
              }
            }
          }
        }
      }
      ... on PullRequest {
        timelineItems(first: 100, itemTypes: [CROSS_REFERENCED_EVENT, CONNECTED_EVENT]) {
          nodes {
            __typename
            ... on CrossReferencedEvent {
              source {
                __typename
                ... on Issue { number }
                ... on PullRequest { number }
              }
            }
            ... on ConnectedEvent {
              subject {
                __typename
                ... on Issue { number }
                ... on PullRequest { number }
              }
            }
          }
        }
      }
    }
  }
}`

	variables := map[string]interface{}{
		"owner":  owner,
		"repo":   repo,
		"number": issueNum,
	}

	var raw rawTimelineResponse
	if err := gql.Do(query, variables, &raw); err != nil {
		if isTimelineSchemaError(err.Error()) {
			return nil, nil
		}
		return nil, err
	}
	return convertTimelineItems(raw.Repository.IssueOrPullRequest.TimelineItems.Nodes), nil
}

// ─── Conversion helpers ───────────────────────────────────────────────────────

func convertNode(n rawNode, kind string) Node {
	labels := make([]string, 0, len(n.Labels.Nodes))
	for _, l := range n.Labels.Nodes {
		labels = append(labels, l.Name)
	}

	assignees := make([]string, 0, len(n.Assignees.Nodes))
	for _, a := range n.Assignees.Nodes {
		assignees = append(assignees, a.Login)
	}

	return Node{
		ID:        n.ID,
		Number:    n.Number,
		Kind:      kind,
		State:     n.State,
		Title:     n.Title,
		URL:       n.URL,
		Labels:    labels,
		Milestone: n.Milestone.Title,
		Assignees: assignees,
		IsDraft:   n.IsDraft,
	}
}

func convertIssuesPage(raw rawIssuesPage) IssuesPage {
	issues := make([]Node, 0, len(raw.Repository.Issues.Nodes))
	for _, n := range raw.Repository.Issues.Nodes {
		issues = append(issues, convertNode(n, "issue"))
	}

	prs := make([]Node, 0, len(raw.Repository.PRs.Nodes))
	for _, n := range raw.Repository.PRs.Nodes {
		prs = append(prs, convertNode(n, "pull_request"))
	}

	hasNext := raw.Repository.Issues.PageInfo.HasNextPage ||
		raw.Repository.PRs.PageInfo.HasNextPage

	cursor := raw.Repository.Issues.PageInfo.EndCursor
	if cursor == "" {
		cursor = raw.Repository.PRs.PageInfo.EndCursor
	}

	return IssuesPage{
		Issues:      issues,
		PRs:         prs,
		HasNextPage: hasNext,
		Cursor:      cursor,
	}
}

func convertSubIssueResult(raw rawSubIssueResponse) SubIssueResult {
	issueNode := raw.Repository.Issue.rawNode
	parent := convertNode(issueNode, "issue")

	children := make([]Node, 0, len(raw.Repository.Issue.SubIssues.Nodes))
	for _, n := range raw.Repository.Issue.SubIssues.Nodes {
		children = append(children, convertNode(n, "issue"))
	}

	return SubIssueResult{
		Parent:   &parent,
		Children: children,
	}
}

func convertTimelineItems(items []rawTimelineItem) []TimelineItem {
	result := make([]TimelineItem, 0, len(items))
	for _, item := range items {
		ti := TimelineItem{
			ItemType:     item.Typename,
			SourceNumber: item.Source.Number,
			TargetNumber: item.Subject.Number,
		}
		result = append(result, ti)
	}
	return result
}

// ─── Schema error detection ───────────────────────────────────────────────────

// isSubIssueSchemaError returns true when the error message indicates the
// GraphQL schema does not support the subIssues / parent fields.
func isSubIssueSchemaError(msg string) bool {
	return strings.Contains(msg, "doesn't exist on type")
}

// isTimelineSchemaError returns true when the error indicates an incompatible
// timeline schema (e.g. unsupported itemTypes enum values).
func isTimelineSchemaError(msg string) bool {
	return strings.Contains(msg, "doesn't exist on type") ||
		strings.Contains(msg, "not a valid value")
}
