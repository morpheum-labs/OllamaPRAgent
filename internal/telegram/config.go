package telegram

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/morpheum-labs/OllamaPRAgent/internal/envx"
	"github.com/morpheum-labs/OllamaPRAgent/internal/gitprovider"
	"github.com/morpheum-labs/OllamaPRAgent/internal/ollama"
	"github.com/morpheum-labs/OllamaPRAgent/internal/review"
)

// AppConfig is runtime configuration for the Telegram bot process.
type AppConfig struct {
	BotToken       string
	DBPath         string
	AllowedChatIDs map[int64]struct{}
	Review         ReviewDefaults
}

// ReviewDefaults mirrors CLI review settings (Gitea/git/file) for bot-triggered runs.
type ReviewDefaults struct {
	Provider       gitprovider.ProviderType
	RepoRoot       string
	DefaultRepo    string
	PromptTemplate string
	PostComment    bool

	DiffPath    string
	BodyPath    string
	CommitsPath string
	GitBase     string
	GitHead     string
	GitTitle    string

	GiteaURL   string
	GiteaToken string

	OllamaURL    string
	OllamaModel  string
	OllamaTemp   float64
	OllamaTopP   float64
	OllamaMaxTok int
}

// LoadAppConfig reads bot and review settings from the environment.
func LoadAppConfig() (AppConfig, error) {
	token := strings.TrimSpace(os.Getenv("ORB_TELEGRAM_BOT_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	}
	if token == "" {
		return AppConfig{}, fmt.Errorf("set ORB_TELEGRAM_BOT_TOKEN or TELEGRAM_BOT_TOKEN")
	}

	dbPath := strings.TrimSpace(os.Getenv("ORB_TELEGRAM_DB_PATH"))
	if dbPath == "" {
		dataDir := strings.TrimSpace(os.Getenv("ORB_TELEGRAM_DATA_DIR"))
		if dataDir == "" {
			dataDir = ".telegram-data"
		}
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			return AppConfig{}, fmt.Errorf("create data dir: %w", err)
		}
		dbPath = filepath.Join(dataDir, "state.db")
	}

	allowed, err := parseAllowedChatIDs(os.Getenv("ORB_TELEGRAM_ALLOWED_CHAT_IDS"))
	if err != nil {
		return AppConfig{}, err
	}

	postComment := envx.EnvOrDefault(true, "ORB_TELEGRAM_POST_COMMENT", "ORB_POST_COMMENT")

	reviewDef := ReviewDefaults{
		Provider:       gitprovider.ProviderType(strings.TrimSpace(os.Getenv("ORB_PROVIDER"))),
		RepoRoot:       envx.EnvOrDefault(".", "ORB_REPO_ROOT"),
		DefaultRepo:    envx.EnvOrDefault("", "ORB_REPO_NAME", "GITHUB_REPOSITORY"),
		PromptTemplate: os.Getenv("ORB_PROMPT_TEMPLATE"),
		PostComment:    postComment,

		DiffPath:    os.Getenv("ORB_FILE_DIFF"),
		BodyPath:    os.Getenv("ORB_FILE_PR_BODY"),
		CommitsPath: os.Getenv("ORB_FILE_COMMITS"),
		GitBase:     envx.EnvOrDefault("origin/main", "ORB_GIT_BASE"),
		GitHead:     envx.EnvOrDefault("HEAD", "ORB_GIT_CURRENT"),
		GitTitle:    os.Getenv("ORB_GIT_TITLE_FILE"),

		GiteaURL:   os.Getenv("ORB_GITEA_URL"),
		GiteaToken: envx.EnvOrDefault("", "ORB_GITEA_TOKEN", "GITHUB_TOKEN"),

		OllamaURL:    envx.OllamaURL(),
		OllamaModel:  envx.EnvOrDefault(review.DefaultOllamaModel, "ORB_OLLAMA_MODEL", "OLLAMA_MODEL"),
		OllamaTemp:   envx.EnvOrDefault(review.DefaultOllamaTemp, "ORB_OLLAMA_TEMPERATURE", "OLLAMA_TEMPERATURE"),
		OllamaTopP:   envx.EnvOrDefault(review.DefaultOllamaTopP, "ORB_OLLAMA_TOP_P", "OLLAMA_TOP_P"),
		OllamaMaxTok: envx.EnvOrDefault(review.DefaultOllamaMaxTokens, "ORB_OLLAMA_MAX_TOKENS", "OLLAMA_MAX_TOKENS"),
	}

	return AppConfig{
		BotToken:       token,
		DBPath:         dbPath,
		AllowedChatIDs: allowed,
		Review:         reviewDef,
	}, nil
}

func parseAllowedChatIDs(raw string) (map[int64]struct{}, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	out := make(map[int64]struct{})
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("ORB_TELEGRAM_ALLOWED_CHAT_IDS: invalid id %q", p)
		}
		out[id] = struct{}{}
	}
	return out, nil
}

// GitProviderConfig builds a gitprovider.Config for the given PR, using defaults plus overrides.
func (d ReviewDefaults) GitProviderConfig(repo string, pr int) gitprovider.Config {
	return gitprovider.Config{
		ProviderType: d.Provider,
		RepoRoot:     d.RepoRoot,
		DiffPath:     d.DiffPath,
		BodyPath:     d.BodyPath,
		CommitsPath:  d.CommitsPath,
		BaseBranch:   d.GitBase,
		HeadBranch:   d.GitHead,
		TitleFile:    d.GitTitle,
		GiteaURL:     d.GiteaURL,
		GiteaToken:   d.GiteaToken,
		RepoName:     repo,
		PRNumber:     pr,
	}
}

// BuildReviewOptions assembles review.Options for a concrete repo/PR and optional per-chat model override.
func (d ReviewDefaults) BuildReviewOptions(repo string, pr int, modelOverride string, onProgress func(string)) review.Options {
	model := d.OllamaModel
	if strings.TrimSpace(modelOverride) != "" {
		model = strings.TrimSpace(modelOverride)
	}
	return review.Options{
		ProviderConfig: d.GitProviderConfig(repo, pr),
		RepoRoot:       d.RepoRoot,
		PromptTemplate: d.PromptTemplate,
		PostComment:    d.PostComment,
		OnProgress:     onProgress,
		Ollama: ollama.RequestOptions{
			URL:         d.OllamaURL,
			Model:       model,
			Temperature: d.OllamaTemp,
			TopP:        d.OllamaTopP,
			NumTokens:   d.OllamaMaxTok,
		},
	}
}
