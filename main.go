package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/morpheum-labs/OllamaPRAgent/internal/gitprovider"
	"github.com/morpheum-labs/OllamaPRAgent/internal/ollama"
	"github.com/morpheum-labs/OllamaPRAgent/internal/prompt"
)

const (
	defaultOllamaTopP      = 0.9
	defaultOllamaTemp      = 0.2
	defaultOllamaMaxTokens = 2048
	defaultOllamaModel     = "qwen2.5-coder:7b"
)

func EnvOrDefault[T float64 | int | bool | string](d T, envs ...string) T {
	for _, env := range envs {
		if envVal := os.Getenv(env); envVal != "" {
			switch any(d).(type) {
			case int:
				if ival, err := strconv.Atoi(envVal); err == nil {
					return any(ival).(T)
				} else {
					fmt.Printf("failed to parse %s as an int. Using default %v", envVal, d)
				}
			case float64:
				if fval, err := strconv.ParseFloat(envVal, 64); err == nil {
					return any(fval).(T)
				} else {
					fmt.Printf("failed to parse %s as an float. Using default %v", envVal, d)
				}
			case bool:
				if bval, err := strconv.ParseBool(envVal); err == nil {
					return any(bval).(T)
				} else {
					fmt.Printf("failed to parse %s as a bool. Using default %v", envVal, d)
				}
			case string:
				return any(envVal).(T)
			default:
				panic(fmt.Sprintf("Unsupported type %T for EnvOrDefault", d))
			}
		}
	}

	return d
}

func ollamaDefaultURL() string {
	if url := os.Getenv("ORB_OLLAMA_URL"); url != "" {
		return url
	}

	if url := os.Getenv("OLLAMA_URL"); url != "" {
		return url
	}

	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		return "http://" + host
	}

	return "http://localhost:11434"
}

func defaultPRNumber() int {
	if prNumStr := os.Getenv("ORB_PR_NUMBER"); prNumStr != "" {
		if prNum, err := strconv.Atoi(prNumStr); err == nil {
			return prNum
		} else {
			fmt.Printf("Failed to parse ORB_PR_NUMBER as an int")
		}
	} else if ghRef := os.Getenv("GITHUB_REF"); ghRef != "" {
		// GITHUB_REF is expected to be in the format "refs/pull/<PR_NUMBER>/merge"
		// We need to extract the PR number from it
		gfPRIndex := 2
		ghRefParts := strings.Split(ghRef, "/")

		if len(ghRefParts) > gfPRIndex {
			if prNum, err := strconv.Atoi(ghRefParts[gfPRIndex]); err == nil {
				return prNum
			} else {
				fmt.Printf("Failed to parse PR number from GITHUB_REF as an int")
			}
		}
	}

	return 0
}

func main() {
	var (
		// Common flags
		postComment    = flag.Bool("post-comment", EnvOrDefault(false, "ORB_POST_COMMENT"), "Post the review as a comment")
		prNumber       = flag.Int("pr-number", defaultPRNumber(), "PR number")
		promptTemplate = flag.String("prompt-template", os.Getenv("ORB_PROMPT_TEMPLATE"), "Path to the prompt template")
		repoRoot       = flag.String("repo-root", EnvOrDefault(".", "ORB_REPO_ROOT"), "Path to the local repo root")
		repoName       = flag.String("repo-name", EnvOrDefault("", "ORB_REPO_NAME", "GITHUB_REPOSITORY"), "Repository identifier (owner/repo)")

		// Ollama settings
		ollamaURL    = flag.String("ollama-url", ollamaDefaultURL(), "Base URL of the Ollama API")
		ollamaModel  = flag.String("ollama-model", EnvOrDefault(defaultOllamaModel, "ORB_OLLAMA_MODEL", "OLLAMA_MODEL"), "Model to use with Ollama")
		ollamaTemp   = flag.Float64("ollama-temperature", EnvOrDefault(defaultOllamaTemp, "ORB_OLLAMA_TEMPERATURE", "OLLAMA_TEMPERATURE"), "Temperature for Ollama prompt generation")
		ollamaTopP   = flag.Float64("ollama-top-p", EnvOrDefault(defaultOllamaTopP, "ORB_OLLAMA_TOP_P", "OLLAMA_TOP_P"), "Top-p (nucleus sampling) value")
		ollamaMaxTok = flag.Int("ollama-max-tokens", EnvOrDefault(defaultOllamaMaxTokens, "ORB_OLLAMA_MAX_TOKENS", "OLLAMA_MAX_TOKENS"), "Maximum number of tokens to generate")

		// Provider selection
		provider = flag.String("provider", os.Getenv("ORB_PROVIDER"), "Which Git provider to use: auto | file | git | gitea")

		// File provider
		diffFile    = flag.String("file-diff", os.Getenv("ORB_FILE_DIFF"), "Path to the diff file (for file provider)")
		prBodyFile  = flag.String("file-pr-body", os.Getenv("ORB_FILE_PR_BODY"), "Path to the PR body file (for file provider)")
		commitsFile = flag.String("file-commits", os.Getenv("ORB_FILE_COMMITS"), "Path to the commits file (for file provider)")

		// Git provider
		gitBase      = flag.String("git-base", EnvOrDefault("origin/main", "ORB_GIT_BASE"), "Base commit for git diff (for git provider)")
		gitCurrent   = flag.String("git-current", EnvOrDefault("HEAD", "ORB_GIT_CURRENT"), "Current commit for git diff (for git provider)")
		gitTitleFile = flag.String("git-title-file", os.Getenv("ORB_GIT_TITLE_FILE"), "Path to a file containing PR title (for git provider)")

		// Gitea provider
		giteaURL   = flag.String("gitea-url", os.Getenv("ORB_GITEA_URL"), "Base URL for Gitea")
		giteaToken = flag.String("gitea-token", EnvOrDefault("", "ORB_GITEA_TOKEN", "GITHUB_TOKEN"), "API token for Gitea")
	)

	flag.Parse()

	// Create provider config
	providerConfig := gitprovider.Config{
		ProviderType: gitprovider.ProviderType(*provider),
		RepoRoot:     *repoRoot,
		DiffPath:     *diffFile,
		BodyPath:     *prBodyFile,
		CommitsPath:  *commitsFile,
		BaseBranch:   *gitBase,
		HeadBranch:   *gitCurrent,
		TitleFile:    *gitTitleFile,
		GiteaURL:     *giteaURL,
		GiteaToken:   *giteaToken,
		RepoName:     *repoName,
		PRNumber:     *prNumber,
	}

	// Create provider
	gitProvider, err := gitprovider.CreateProvider(providerConfig)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	// Get PR context
	prContext, err := gitProvider.GetPRContext()
	if err != nil {
		log.Fatalf("Failed to get PR context: %v", err)
	}

	// Build prompt from context
	promptData, err := prompt.BuildPromptFromContext(prContext, *repoRoot, *promptTemplate)
	if err != nil {
		log.Fatalf("Failed to build prompt: %v", err)
	}

	// Send to Ollama
	response, err := ollama.SendPrompt(ollama.RequestOptions{
		URL:         *ollamaURL,
		Model:       *ollamaModel,
		Prompt:      promptData,
		Temperature: *ollamaTemp,
		TopP:        *ollamaTopP,
		NumTokens:   *ollamaMaxTok,
	})
	if err != nil {
		log.Fatalf("ollama call failed: %v", err)
	}

	// Post comment if requested, otherwise print response
	if *postComment {
		if err := gitProvider.PostComment(response); err != nil {
			log.Printf("Warning: Failed to post comment: %v", err)
		}
	} else {
		fmt.Println("=== Review Response ===")
		fmt.Println(response)
	}
}
