package gitprovider

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v62/github"
	"golang.org/x/oauth2"
)

// GitHubProvider loads PR context via the GitHub REST API and posts issue comments.
type GitHubProvider struct {
	client *github.Client
	owner  string
	repo   string
	prNum  int
}

// NewGitHubProvider builds a provider for github.com or GitHub Enterprise (GHES).
// fullName must be "owner/repo".
func NewGitHubProvider(token, baseURL, fullName string, pr int) (GitProvider, error) {
	if token == "" {
		return nil, fmt.Errorf("github token is empty")
	}
	parts := strings.Split(fullName, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid repo name %q (want owner/repo)", fullName)
	}
	if pr <= 0 {
		return nil, fmt.Errorf("invalid PR number %d", pr)
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	hc := oauth2.NewClient(ctx, ts)

	var client *github.Client
	var err error
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		client = github.NewClient(hc)
	} else {
		baseURL = strings.TrimSuffix(baseURL, "/")
		client, err = github.NewEnterpriseClient(baseURL, baseURL, hc)
		if err != nil {
			return nil, fmt.Errorf("github enterprise client: %w", err)
		}
	}

	return &GitHubProvider{
		client: client,
		owner:  parts[0],
		repo:   parts[1],
		prNum:  pr,
	}, nil
}

// GetPRContext fetches PR metadata, commits, and unified diff from GitHub.
func (g *GitHubProvider) GetPRContext() (*PRContext, error) {
	ctx := context.Background()

	pr, _, err := g.client.PullRequests.Get(ctx, g.owner, g.repo, g.prNum)
	if err != nil {
		return nil, fmt.Errorf("github get pull request: %w", err)
	}

	diffText, _, err := g.client.PullRequests.GetRaw(ctx, g.owner, g.repo, g.prNum, github.RawOptions{Type: github.Diff})
	if err != nil {
		return nil, fmt.Errorf("github get diff: %w", err)
	}

	var commitMsgs []string
	listOpts := &github.ListOptions{PerPage: 100}
	for {
		commits, resp, err := g.client.PullRequests.ListCommits(ctx, g.owner, g.repo, g.prNum, listOpts)
		if err != nil {
			return nil, fmt.Errorf("github list commits: %w", err)
		}
		for _, c := range commits {
			if c.Commit != nil && c.Commit.Message != nil {
				commitMsgs = append(commitMsgs, *c.Commit.Message)
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		listOpts.Page = resp.NextPage
	}

	headRef := ""
	baseRef := ""
	title := ""
	body := ""
	if pr.Head != nil && pr.Head.Ref != nil {
		headRef = *pr.Head.Ref
	}
	if pr.Base != nil && pr.Base.Ref != nil {
		baseRef = *pr.Base.Ref
	}
	if pr.Title != nil {
		title = *pr.Title
	}
	if pr.Body != nil {
		body = *pr.Body
	}

	return &PRContext{
		PRNumber:   g.prNum,
		Repository: fmt.Sprintf("%s/%s", g.owner, g.repo),
		HeadBranch: headRef,
		BaseBranch: baseRef,
		Title:      title,
		Body:       body,
		Commits:    commitMsgs,
		Diff:       diffText,
	}, nil
}

// PostComment adds an issue comment on the pull request.
func (g *GitHubProvider) PostComment(comment string) error {
	ctx := context.Background()
	_, _, err := g.client.Issues.CreateComment(ctx, g.owner, g.repo, g.prNum, &github.IssueComment{Body: github.String(comment)})
	if err != nil {
		return fmt.Errorf("github create comment: %w", err)
	}
	return nil
}
