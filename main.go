package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/morpheum-labs/OllamaPRAgent/internal/envx"
	"github.com/morpheum-labs/OllamaPRAgent/internal/gitprovider"
	"github.com/morpheum-labs/OllamaPRAgent/internal/ollama"
	"github.com/morpheum-labs/OllamaPRAgent/internal/review"
)

// Set at link time via -ldflags (see Makefile).
var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	var (
		showVersion = flag.Bool("version", false, "Print version and exit")

		postComment    = flag.Bool("post-comment", envx.EnvOrDefault(false, "ORB_POST_COMMENT"), "Post the review as a comment")
		prNumber       = flag.Int("pr-number", envx.PRNumber(), "PR number")
		promptTemplate = flag.String("prompt-template", os.Getenv("ORB_PROMPT_TEMPLATE"), "Path to the prompt template")
		repoRoot       = flag.String("repo-root", envx.EnvOrDefault(".", "ORB_REPO_ROOT"), "Path to the local repo root")
		repoName       = flag.String("repo-name", envx.EnvOrDefault("", "ORB_REPO_NAME", "GITHUB_REPOSITORY"), "Repository identifier (owner/repo)")

		ollamaURL    = flag.String("ollama-url", envx.OllamaURL(), "Base URL of the Ollama API")
		ollamaModel  = flag.String("ollama-model", envx.EnvOrDefault(review.DefaultOllamaModel, "ORB_OLLAMA_MODEL", "OLLAMA_MODEL"), "Model to use with Ollama")
		ollamaTemp   = flag.Float64("ollama-temperature", envx.EnvOrDefault(review.DefaultOllamaTemp, "ORB_OLLAMA_TEMPERATURE", "OLLAMA_TEMPERATURE"), "Temperature for Ollama prompt generation")
		ollamaTopP   = flag.Float64("ollama-top-p", envx.EnvOrDefault(review.DefaultOllamaTopP, "ORB_OLLAMA_TOP_P", "OLLAMA_TOP_P"), "Top-p (nucleus sampling) value")
		ollamaMaxTok = flag.Int("ollama-max-tokens", envx.EnvOrDefault(review.DefaultOllamaMaxTokens, "ORB_OLLAMA_MAX_TOKENS", "OLLAMA_MAX_TOKENS"), "Maximum number of tokens to generate")

		provider = flag.String("provider", os.Getenv("ORB_PROVIDER"), "Which Git provider to use: auto | file | git | gitea | github")

		diffFile    = flag.String("file-diff", os.Getenv("ORB_FILE_DIFF"), "Path to the diff file (for file provider)")
		prBodyFile  = flag.String("file-pr-body", os.Getenv("ORB_FILE_PR_BODY"), "Path to the PR body file (for file provider)")
		commitsFile = flag.String("file-commits", os.Getenv("ORB_FILE_COMMITS"), "Path to the commits file (for file provider)")

		gitBase      = flag.String("git-base", envx.EnvOrDefault("origin/main", "ORB_GIT_BASE"), "Base commit for git diff (for git provider)")
		gitCurrent   = flag.String("git-current", envx.EnvOrDefault("HEAD", "ORB_GIT_CURRENT"), "Current commit for git diff (for git provider)")
		gitTitleFile = flag.String("git-title-file", os.Getenv("ORB_GIT_TITLE_FILE"), "Path to a file containing PR title (for git provider)")

		giteaURL   = flag.String("gitea-url", os.Getenv("ORB_GITEA_URL"), "Base URL for Gitea")
		giteaToken = flag.String("gitea-token", envx.EnvOrDefault("", "ORB_GITEA_TOKEN", "GITHUB_TOKEN"), "API token for Gitea")

		githubToken   = flag.String("github-token", envx.EnvOrDefault("", "GITHUB_PAT", "GITHUB_TOKEN", "ORB_GITHUB_TOKEN"), "PAT for GitHub provider")
		githubAPIURL  = flag.String("github-api-url", envx.EnvOrDefault("", "GITHUB_API_URL", "ORB_GITHUB_BASE_URL"), "GitHub API base URL (empty = github.com; GHES: https://host/api/v3)")
	)

	flag.Parse()

	if *showVersion {
		fmt.Printf("%s\n", version)
		fmt.Printf("commit %s\n", commit)
		fmt.Printf("built %s\n", buildTime)
		return
	}

	providerConfig := gitprovider.Config{
		ProviderType: gitprovider.ProviderType(*provider),
		RepoRoot:     *repoRoot,
		DiffPath:     *diffFile,
		BodyPath:     *prBodyFile,
		CommitsPath:  *commitsFile,
		BaseBranch:   *gitBase,
		HeadBranch:   *gitCurrent,
		TitleFile:    *gitTitleFile,
		GiteaURL:      *giteaURL,
		GiteaToken:    *giteaToken,
		GitHubToken:   *githubToken,
		GitHubBaseURL: *githubAPIURL,
		RepoName:      *repoName,
		PRNumber:      *prNumber,
	}

	res, err := review.Run(context.Background(), review.Options{
		ProviderConfig: providerConfig,
		RepoRoot:       *repoRoot,
		PromptTemplate: *promptTemplate,
		PostComment:    *postComment,
		Ollama: ollama.RequestOptions{
			URL:         *ollamaURL,
			Model:       *ollamaModel,
			Temperature: *ollamaTemp,
			TopP:        *ollamaTopP,
			NumTokens:   *ollamaMaxTok,
		},
	})
	if err != nil {
		log.Fatalf("review failed: %v", err)
	}

	if !*postComment {
		fmt.Println("=== Review Response ===")
		fmt.Println(res.Review)
	}
}
