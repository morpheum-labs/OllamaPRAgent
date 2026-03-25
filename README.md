# Ollama Review Bot

This is a review bot designed for using Ollama to provide feedback on a diff or pull request.

## Features
- Uses the Ollama API to generate reviews based on the provided diff or pull request.
- Supports multiple languages and file types.
- Can provide feedback using flat files, git commands, or via the Gitea API.
- Posts feedback as review comments when supported by the provider.
- Highly configurable with various options for customization:
  - Common options: PR number, prompt template, repository root, repository name
  - Ollama settings: model selection, temperature, top-p value, maximum tokens
  - Provider selection: file, git, gitea
  - File provider options: diff file, PR body file, commits file
  - Git provider options: base commit, current commit, title file
  - Gitea provider options: base URL, API token

## Usage
The bot can be configured through command-line flags to tailor its behavior for different use cases.

### Common Flags
- `-post-comment`: Post the review as a comment. Example: `./ollama-review-bot -post-comment=true`
- `-pr-number <int>`: PR number. Example: `./ollama-review-bot -pr-number=123`
- `-prompt-template <string>`: Path to the prompt template. Example: `./ollama-review-bot -prompt-template=/path/to/template.txt`
- `-repo-root <string>`: Path to the local repo root. Default is `.`. Example: `./ollama-review-bot -repo-root=/path/to/repo`
- `-repo-name <string>`: Repository identifier (owner/repo). Example: `./ollama-review-bot -repo-name=username/repository`

### Ollama Settings
- `-ollama-url <string>`: Base URL of the Ollama API. Default is `http://localhost:11434`. Example: `./ollama-review-bot -ollama-url=http://example.com/api`
- `-ollama-model <string>`: Model to use with Ollama. Default is `qwen2.5-coder:7b`. Example: `./ollama-review-bot -ollama-model=my-custom-model`
- `-ollama-temperature <float>`: Temperature for Ollama prompt generation. Default is 0.2. Example: `./ollama-review-bot -ollama-temperature=0.8`
- `-ollama-top-p <float>`: Top-p (nucleus sampling) value. Default is 0.9. Example: `./ollama-review-bot -ollama-top-p=0.75`
- `-ollama-max-tokens <int>`: Maximum number of tokens to generate. Default is 2048. Example: `./ollama-review-bot -ollama-max-tokens=1024`

### Provider Selection
- `-provider <string>`: Which Git provider to use (auto | file | git | gitea). Example: `./ollama-review-bot -provider=gitea`

### File Provider Options
- `-file-diff <string>`: Path to the diff file. Example: `./ollama-review-bot -file-diff=/path/to/diff.txt`
- `-file-pr-body <string>`: Path to the PR body file. Example: `./ollama-review-bot -file-pr-body=/path/to/prbody.txt`
- `-file-commits <string>`: Path to the commits file. Example: `./ollama-review-bot -file-commits=/path/to/commits.txt`

### Git Provider Options
- `-git-base <string>`: Base commit for git diff. Default is `origin/main`. Example: `./ollama-review-bot -git-base=origin/develop`
- `-git-current <string>`: Current commit for git diff. Default is `HEAD`. Example: `./ollama-review-bot -git-current=feature-branch`
- `-git-title-file <string>`: Path to a file containing PR title. Example: `./ollama-review-bot -git-title-file=/path/to/title.txt`

### Gitea Provider Options
- `-gitea-url <string>`: Base URL for Gitea. Example: `./ollama-review-bot -gitea-url=http://gitea.example.com`
- `-gitea-token <string>`: API token for Gitea. Example: `./ollama-review-bot -gitea-token=your_api_token`

These examples demonstrate how to configure the bot using various command-line flags to suit different use cases.
