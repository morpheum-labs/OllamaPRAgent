package gitprovider

import (
	"fmt"
	"strings"
)

// ProviderType represents the type of Git provider
type ProviderType string

const (
	// FileProviderType represents a file-based provider
	FileProviderType ProviderType = "file"
	// GitProviderType represents a local git-based provider
	GitProviderType ProviderType = "git"
	// GiteaProviderType represents a Gitea-based provider
	GiteaProviderType ProviderType = "gitea"
	// GitHubProviderType uses the GitHub REST API (github.com or GHES).
	GitHubProviderType ProviderType = "github"
	// AutoProviderType indicates that the provider should be auto-detected
	AutoProviderType ProviderType = "auto"
)

// Config holds configuration for all possible providers
type Config struct {
	// General
	ProviderType ProviderType
	RepoRoot     string

	// Common options
	RepoName string
	PRNumber int

	// File provider
	DiffPath    string
	BodyPath    string
	CommitsPath string

	// Git provider
	BaseBranch string
	HeadBranch string
	TitleFile  string

	// Gitea provider
	GiteaURL   string
	GiteaToken string

	// GitHub provider (PAT with repo + pull_requests read/write for private repos)
	GitHubToken   string
	GitHubBaseURL string // empty = https://api.github.com; GHES: e.g. https://github.mycompany.com/api/v3
}

// CreateProvider creates the appropriate provider based on configuration
func CreateProvider(cfg Config) (GitProvider, error) {
	// If Auto provider type, detect the provider based on available information
	if cfg.ProviderType == AutoProviderType || cfg.ProviderType == "" {
		cfg.ProviderType = detectProviderType(cfg)
	}

	switch cfg.ProviderType {
	case FileProviderType:
		if cfg.DiffPath == "" || cfg.BodyPath == "" || cfg.CommitsPath == "" {
			return nil, fmt.Errorf("file provider requires diff, body, and commits file paths")
		}

		return NewFileProvider(cfg.DiffPath, cfg.BodyPath, cfg.CommitsPath, cfg.RepoName, cfg.PRNumber), nil

	case GitProviderType:
		if cfg.RepoRoot == "" {
			return nil, fmt.Errorf("git provider requires repo root")
		}

		if cfg.BaseBranch == "" {
			cfg.BaseBranch = "origin/main"
		}

		if cfg.HeadBranch == "" {
			cfg.HeadBranch = "HEAD"
		}

		return NewGitProvider(cfg.RepoRoot, cfg.BaseBranch, cfg.HeadBranch, cfg.RepoName, cfg.PRNumber, cfg.TitleFile, cfg.BodyPath), nil

	case GiteaProviderType:
		var missingVars []string

		if cfg.GiteaURL == "" {
			missingVars = append(missingVars, "-gitea-url")
		}

		if cfg.GiteaToken == "" {
			missingVars = append(missingVars, "-gitea-token")
		}

		if cfg.RepoName == "" {
			missingVars = append(missingVars, "-repo-name")
		}

		if cfg.PRNumber == 0 {
			missingVars = append(missingVars, "-pr-number")
		}

		if len(missingVars) > 0 {
			return nil, fmt.Errorf("gitea provider requires the following variables: %s", missingVars)
		}

		return NewGiteaProvider(cfg.GiteaURL, cfg.GiteaToken, cfg.RepoName, cfg.PRNumber), nil

	case GitHubProviderType:
		if cfg.GitHubToken == "" {
			return nil, fmt.Errorf("github provider requires a token (GitHub PAT)")
		}
		if cfg.RepoName == "" || !strings.Contains(cfg.RepoName, "/") {
			return nil, fmt.Errorf("github provider requires repo-name as owner/repo")
		}
		if cfg.PRNumber == 0 {
			return nil, fmt.Errorf("github provider requires a non-zero PR number")
		}
		return NewGitHubProvider(cfg.GitHubToken, cfg.GitHubBaseURL, cfg.RepoName, cfg.PRNumber)

	case AutoProviderType:
		return nil, fmt.Errorf("provider type is set to auto but no specific provider was detected")
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.ProviderType)
	}
}

// detectProviderType tries to determine the most appropriate provider based on given configuration
func detectProviderType(cfg Config) ProviderType {
	// Check for Gitea-specific options
	if cfg.GiteaURL != "" && cfg.GiteaToken != "" && cfg.PRNumber != 0 {
		fmt.Println("Detected Gitea provider")
		return GiteaProviderType
	}

	if cfg.GitHubToken != "" && cfg.RepoName != "" && strings.Contains(cfg.RepoName, "/") && cfg.PRNumber != 0 {
		fmt.Println("Detected GitHub provider")
		return GitHubProviderType
	}

	// Check for file-specific options
	if cfg.DiffPath != "" && cfg.BodyPath != "" && cfg.CommitsPath != "" {
		fmt.Println("Detected File provider")
		return FileProviderType
	}

	// Default to Git provider if nothing else matches
	fmt.Println("Detected Git provider")

	return GitProviderType
}
