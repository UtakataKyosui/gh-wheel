// Package ghclient wraps go-gh's REST and GraphQL clients for use by
// gh-wheel subcommands.
//
// Typical usage:
//
//	c, err := ghclient.New(repoFlag)
//	if err != nil { return err }
//	var pr PRInfo
//	if err := c.RepoGet(fmt.Sprintf("pulls/%d", num), &pr); err != nil { return err }
package ghclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/repo"
)

// Client holds a REST client and the resolved owner/repo pair.
// The GraphQL client is created lazily on first use.
type Client struct {
	rest  *api.RESTClient
	owner string
	name  string
}

// New creates a Client for the given repository.
//
// flagRepo is the value of the -R/--repo flag (empty means "detect from cwd").
// Authentication and repo resolution errors are returned as *cliexit.Error.
func New(flagRepo string) (*Client, error) {
	rest, err := api.DefaultRESTClient()
	if err != nil {
		return nil, cliexit.NewAuth(cliexit.ErrCodeAuthNoToken,
			fmt.Errorf("GitHub authentication failed: %w\nRun: gh auth login", err))
	}

	r, err := repo.Resolve(flagRepo)
	if err != nil {
		return nil, err // already *cliexit.Error from repo.Resolve
	}

	return &Client{rest: rest, owner: r.Owner, name: r.Name}, nil
}

// NewForTest creates a Client whose HTTP requests are intercepted by transport.
// The transport should route requests to an httptest server.
// Intended for unit tests in other packages that need to mock GitHub API calls.
//
// Because go-gh always builds https://api.github.com/... URLs, the transport
// must be set up to accept the self-signed TLS certificate of the test server
// AND rewrite the destination URL.  Use TestTransport (below) for convenience.
func NewForTest(transport http.RoundTripper, owner, name string) (*Client, error) {
	rest, err := api.NewRESTClient(api.ClientOptions{
		Transport: transport,
		Host:      "github.com",
		AuthToken: "test-token",
	})
	if err != nil {
		return nil, err
	}
	return &Client{rest: rest, owner: owner, name: name}, nil
}

// TestTransport wraps an httptest.Server's TLS-aware transport and rewrites
// every request URL to point at the test server instead of api.github.com.
type TestTransport struct {
	Inner   http.RoundTripper
	BaseURL string // e.g. "https://127.0.0.1:PORT"
}

// RoundTrip rewrites req.URL.Host and req.URL.Scheme to the test server's
// address, then delegates to the inner transport.
func (t *TestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Host = strings.TrimPrefix(strings.TrimPrefix(t.BaseURL, "https://"), "http://")
	clone.URL.Scheme = "https"
	return t.Inner.RoundTrip(clone)
}

// Owner returns the repository owner.
func (c *Client) Owner() string { return c.owner }

// Name returns the repository name.
func (c *Client) Name() string { return c.name }

// repoPath builds a repos/{owner}/{repo}/{path} endpoint string.
func (c *Client) repoPath(path string) string {
	return fmt.Sprintf("repos/%s/%s/%s", c.owner, c.name, path)
}

// ─── REST delegation ─────────────────────────────────────────────────────────

// Get performs a REST GET request against a full endpoint path (no leading /).
// resp is unmarshalled from the JSON response body.
func (c *Client) Get(path string, resp any) error {
	if err := c.rest.Get(path, resp); err != nil {
		return cliexit.NewAPI(cliexit.ErrCodeAPI, fmt.Errorf("GET %s: %w", path, err))
	}
	return nil
}

// RepoGet performs a REST GET against repos/{owner}/{repo}/{path}.
func (c *Client) RepoGet(path string, resp any) error {
	return c.Get(c.repoPath(path), resp)
}

// Post performs a REST POST request.
// payload is JSON-marshalled and sent as the request body.
func (c *Client) Post(path string, payload any, resp any) error {
	body, err := jsonBody(payload)
	if err != nil {
		return cliexit.NewGeneral(fmt.Errorf("marshal POST body: %w", err))
	}
	if err := c.rest.Post(path, body, resp); err != nil {
		return cliexit.NewAPI(cliexit.ErrCodeAPI, fmt.Errorf("POST %s: %w", path, err))
	}
	return nil
}

// RepoPost performs a REST POST against repos/{owner}/{repo}/{path}.
func (c *Client) RepoPost(path string, payload any, resp any) error {
	return c.Post(c.repoPath(path), payload, resp)
}

// Patch performs a REST PATCH request.
func (c *Client) Patch(path string, payload any, resp any) error {
	body, err := jsonBody(payload)
	if err != nil {
		return cliexit.NewGeneral(fmt.Errorf("marshal PATCH body: %w", err))
	}
	if err := c.rest.Patch(path, body, resp); err != nil {
		return cliexit.NewAPI(cliexit.ErrCodeAPI, fmt.Errorf("PATCH %s: %w", path, err))
	}
	return nil
}

// RepoPatch performs a REST PATCH against repos/{owner}/{repo}/{path}.
func (c *Client) RepoPatch(path string, payload any, resp any) error {
	return c.Patch(c.repoPath(path), payload, resp)
}

// GraphQL returns a GraphQL client for this installation.
// Created on every call (stateless; go-gh handles connection pooling).
func (c *Client) GraphQL() (*api.GraphQLClient, error) {
	gql, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, cliexit.NewAuth(cliexit.ErrCodeAuthNoToken,
			fmt.Errorf("GitHub GraphQL authentication failed: %w", err))
	}
	return gql, nil
}

// ─── Auth user ───────────────────────────────────────────────────────────────

// CurrentUser returns the authenticated GitHub login name.
// Used for self-approve guards and similar checks.
func (c *Client) CurrentUser() (string, error) {
	var resp struct {
		Login string `json:"login"`
	}
	if err := c.rest.Get("user", &resp); err != nil {
		return "", cliexit.NewAPI(cliexit.ErrCodeAPI,
			fmt.Errorf("GET user: %w", err))
	}
	return resp.Login, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func jsonBody(v any) (io.Reader, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
}
