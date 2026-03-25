package gitprovider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GiteaProvider implements GitProvider interface for Gitea repositories
type GiteaProvider struct {
	URL        string
	Token      string
	Repository string
	PRNumber   int
}

// NewGiteaProvider creates a new Gitea-based provider
func NewGiteaProvider(url, token, repository string, prNumber int) *GiteaProvider {
	// Ensure URL doesn't end with a slash
	url = strings.TrimSuffix(url, "/")

	return &GiteaProvider{
		URL:        url,
		Token:      token,
		Repository: repository,
		PRNumber:   prNumber,
	}
}

// GiteaPR represents a Gitea pull request
type GiteaPR struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Head  struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

// GiteaCommit represents a Gitea commit
type GiteaCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
	} `json:"commit"`
}

// GetPRContext fetches PR information from Gitea API
func (g *GiteaProvider) GetPRContext() (*PRContext, error) {
	if g.URL == "" || g.Token == "" || g.Repository == "" || g.PRNumber == 0 {
		return nil, fmt.Errorf("missing required fields for Gitea provider")
	}

	// Fetch PR information
	pr, err := g.getPR()
	if err != nil {
		return nil, err
	}

	// Fetch commits
	commits, err := g.getCommits()
	if err != nil {
		return nil, err
	}

	// Fetch diff
	diff, err := g.getDiff()
	if err != nil {
		return nil, err
	}

	commitMessages := make([]string, len(commits))
	for i, commit := range commits {
		commitMessages[i] = commit.Commit.Message
	}

	return &PRContext{
		PRNumber:   pr.Number,
		Repository: g.Repository,
		HeadBranch: pr.Head.Ref,
		BaseBranch: pr.Base.Ref,
		Title:      pr.Title,
		Body:       pr.Body,
		Commits:    commitMessages,
		Diff:       diff,
	}, nil
}

// PostComment posts a comment to the PR
func (g *GiteaProvider) PostComment(comment string) error {
	// TODO: This currently just posts as a comment, but in the future it could post a review
	// including inline comments.
	url := fmt.Sprintf("%s/api/v1/repos/%s/issues/%d/comments", g.URL, g.Repository, g.PRNumber)

	// Prepare request body
	payload := struct {
		Body string `json:"body"`
	}{
		Body: comment,
	}

	_, err := g.makeRequest("POST", url, payload)
	if err != nil {
		return fmt.Errorf("failed to post comment: %w", err)
	}

	return nil
}

// getPR fetches PR information from Gitea API
func (g *GiteaProvider) getPR() (*GiteaPR, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/pulls/%d", g.URL, g.Repository, g.PRNumber)

	resp, err := g.makeRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get PR, status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var pr GiteaPR
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, fmt.Errorf("failed to unmarshal PR data: %w", err)
	}

	return &pr, nil
}

// getCommits fetches commits in the PR from Gitea API
func (g *GiteaProvider) getCommits() ([]GiteaCommit, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/pulls/%d/commits", g.URL, g.Repository, g.PRNumber)

	resp, err := g.makeRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get commits, status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var commits []GiteaCommit
	if err := json.Unmarshal(body, &commits); err != nil {
		return nil, fmt.Errorf("failed to unmarshal commits data: %w", err)
	}

	return commits, nil
}

// getDiff fetches diff for the PR from Gitea API
func (g *GiteaProvider) getDiff() (string, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/pulls/%d.diff", g.URL, g.Repository, g.PRNumber)

	resp, err := g.makeRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get diff, status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read diff data: %w", err)
	}

	return string(body), nil
}

// makeRequest is a helper function to make authenticated requests to Gitea API
func (g *GiteaProvider) makeRequest(method, url string, body any) (*http.Response, error) {
	var req *http.Request

	var err error

	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}

		req, err = http.NewRequest(method, url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
	}

	// Set authentication header
	req.Header.Set("Authorization", "token "+g.Token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}
