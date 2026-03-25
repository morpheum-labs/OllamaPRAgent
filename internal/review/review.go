package review

import (
	"context"
	"fmt"
	"strings"

	"github.com/morpheum-labs/OllamaPRAgent/internal/gitprovider"
	"github.com/morpheum-labs/OllamaPRAgent/internal/ollama"
	"github.com/morpheum-labs/OllamaPRAgent/internal/prompt"
	"github.com/sourcegraph/go-diff/diff"
)

// Options configures a single review run (CLI, Telegram, or other callers).
type Options struct {
	ProviderConfig gitprovider.Config
	RepoRoot       string
	PromptTemplate string
	Ollama         ollama.RequestOptions
	PostComment    bool
	OnProgress     func(msg string)
}

// Result is the outcome of a review run.
type Result struct {
	Review string
}

func progress(opts Options, msg string) {
	if opts.OnProgress != nil {
		opts.OnProgress(msg)
	}
}

func countChangedFiles(diffText string) int {
	parsed, err := diff.ParseMultiFileDiff([]byte(diffText))
	if err != nil || len(parsed) == 0 {
		return 0
	}
	return len(parsed)
}

// Run fetches PR context, builds the prompt, calls Ollama, and optionally posts the comment.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	progress(opts, "Fetching PR context…")

	p, err := gitprovider.CreateProvider(opts.ProviderConfig)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}

	prCtx, err := p.GetPRContext()
	if err != nil {
		return nil, fmt.Errorf("get PR context: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	n := countChangedFiles(prCtx.Diff)
	if n > 0 {
		progress(opts, fmt.Sprintf("Analyzing %d changed file(s)…", n))
	} else {
		progress(opts, "Building review from diff…")
	}

	progress(opts, "Building review prompt…")

	promptData, err := prompt.BuildPromptFromContext(prCtx, opts.RepoRoot, opts.PromptTemplate)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	progress(opts, "Calling Ollama (this may take a minute)…")

	ollamaOpts := opts.Ollama
	ollamaOpts.Prompt = promptData
	resp, err := ollama.SendPrompt(ollamaOpts)
	if err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if opts.PostComment {
		progress(opts, "Posting comment on the PR…")
		if err := p.PostComment(resp); err != nil {
			return nil, fmt.Errorf("post comment: %w", err)
		}
		progress(opts, "Review posted.")
	} else {
		progress(opts, "Review complete.")
	}

	return &Result{Review: strings.TrimSpace(resp)}, nil
}
