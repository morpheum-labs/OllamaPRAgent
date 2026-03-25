package gitprovider

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GitLocalProvider implements GitProvider interface for local git repositories
type GitLocalProvider struct {
	BaseBranch string
	HeadBranch string
	RepoRoot   string
	Repository string // Optional repository name
	PRNumber   int    // Optional PR number
	TitleFile  string // Optional path to title file
	PRBodyFile string // Optional path to PR body file
}

// NewGitProvider creates a new git-based provider
func NewGitProvider(repoRoot, baseBranch, headBranch string, repository string, prNumber int, titleFile, prBodyFile string) *GitLocalProvider {
	return &GitLocalProvider{
		BaseBranch: baseBranch,
		HeadBranch: headBranch,
		RepoRoot:   repoRoot,
		Repository: repository,
		PRNumber:   prNumber,
		TitleFile:  titleFile,
		PRBodyFile: prBodyFile,
	}
}

// GetPRContext uses git commands to fetch PR information
func (g *GitLocalProvider) GetPRContext() (*PRContext, error) {
	if g.BaseBranch == "" || g.HeadBranch == "" {
		return nil, errors.New("base and head branches must be specified")
	}

	// Get diff between branches
	diffOutput, err := g.gitCommand("diff", g.BaseBranch, g.HeadBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}

	// Get commit messages
	commits, err := g.getCommitMessages(g.BaseBranch, g.HeadBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit messages: %w", err)
	}

	// Get PR title
	title := g.getTitle()

	// Get PR body
	body := g.getBody()

	// Return context
	return &PRContext{
		PRNumber:   g.PRNumber,
		Repository: g.Repository,
		HeadBranch: g.HeadBranch,
		BaseBranch: g.BaseBranch,
		Title:      title,
		Body:       body,
		Commits:    commits,
		Diff:       diffOutput,
	}, nil
}

// PostComment is a no-op for git provider
func (g *GitLocalProvider) PostComment(comment string) error {
	// In git mode, we just print to stdout
	fmt.Println("=== Review Response ===")
	fmt.Println(comment)

	return nil
}

// gitCommand executes a git command and returns its output
func (g *GitLocalProvider) gitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.RepoRoot

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git command failed: %s. Error: %w", stderr.String(), err)
	}

	return stdout.String(), nil
}

// getCommitMessages returns commit messages between base and head
func (g *GitLocalProvider) getCommitMessages(base, head string) ([]string, error) {
	output, err := g.gitCommand("log", "--pretty=format:%s", fmt.Sprintf("%s..%s", base, head))
	if err != nil {
		return nil, err
	}

	var commits []string

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" {
			commits = append(commits, line)
		}
	}

	return commits, nil
}

// getTitle returns a PR title from the file if provided, or from the last commit
func (g *GitLocalProvider) getTitle() string {
	if g.TitleFile != "" {
		content, err := os.ReadFile(g.TitleFile)
		if err == nil && len(content) > 0 {
			return strings.TrimSpace(string(content))
		}
	}

	// If no title file or it failed to read, use the latest commit message
	title, err := g.gitCommand("log", "-1", "--pretty=format:%s", g.HeadBranch)
	if err == nil {
		return strings.TrimSpace(title)
	}

	return "Pull Request"
}

// getBody returns a PR body from the file if provided, or empty
func (g *GitLocalProvider) getBody() string {
	if g.PRBodyFile != "" {
		content, err := os.ReadFile(g.PRBodyFile)
		if err == nil {
			return string(content)
		}
	}

	return ""
}
