package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/morpheum-labs/OllamaPRAgent/internal/review"
)

// Server holds HTTP server and review settings loaded from the environment.
type Server struct {
	Port           string
	PublicURL      string
	DBPath         string
	GitHubPAT      string
	GitHubBaseURL  string
	AdminToken     string
	PromptTemplate string
	RepoRoot       string
	PostComment    bool
	OllamaURL      string
	OllamaModel    string
	OllamaTemp     float64
	OllamaTopP     float64
	OllamaMaxTok   int
}

// serverFile is the on-disk shape for JSON/TOML. Pointer fields mean “omit to keep previous value”.
type serverFile struct {
	Port           *string  `json:"port,omitempty" toml:"port,omitempty"`
	PublicURL      *string  `json:"public_url,omitempty" toml:"public_url,omitempty"`
	DBPath         *string  `json:"db_path,omitempty" toml:"db_path,omitempty"`
	GitHubPAT      *string  `json:"github_pat,omitempty" toml:"github_pat,omitempty"`
	GitHubBaseURL  *string  `json:"github_base_url,omitempty" toml:"github_base_url,omitempty"`
	AdminToken     *string  `json:"admin_token,omitempty" toml:"admin_token,omitempty"`
	PromptTemplate *string  `json:"prompt_template,omitempty" toml:"prompt_template,omitempty"`
	RepoRoot       *string  `json:"repo_root,omitempty" toml:"repo_root,omitempty"`
	PostComment    *bool    `json:"post_comment,omitempty" toml:"post_comment,omitempty"`
	OllamaURL      *string  `json:"ollama_url,omitempty" toml:"ollama_url,omitempty"`
	OllamaModel    *string  `json:"ollama_model,omitempty" toml:"ollama_model,omitempty"`
	OllamaTemp     *float64 `json:"ollama_temperature,omitempty" toml:"ollama_temperature,omitempty"`
	OllamaTopP     *float64 `json:"ollama_top_p,omitempty" toml:"ollama_top_p,omitempty"`
	OllamaMaxTok   *int     `json:"ollama_max_tokens,omitempty" toml:"ollama_max_tokens,omitempty"`
}

func serverDefaults() Server {
	return Server{
		Port:           "8080",
		PublicURL:      "",
		DBPath:         "./data/watchlist.db",
		GitHubPAT:      "",
		GitHubBaseURL:  "",
		AdminToken:     "",
		PromptTemplate: "",
		RepoRoot:       ".",
		PostComment:    true,
		OllamaURL:      "http://localhost:11434",
		OllamaModel:    review.DefaultOllamaModel,
		OllamaTemp:     review.DefaultOllamaTemp,
		OllamaTopP:     review.DefaultOllamaTopP,
		OllamaMaxTok:   review.DefaultOllamaMaxTokens,
	}
}

func firstNonEmptyEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func applyServerEnvOverrides(s *Server) {
	if v := firstNonEmptyEnv("PORT"); v != "" {
		s.Port = strings.TrimPrefix(strings.TrimSpace(v), ":")
	}
	if v := firstNonEmptyEnv("GITHUB_PAT", "GITHUB_TOKEN"); v != "" {
		s.GitHubPAT = v
	}
	if v := firstNonEmptyEnv("DB_PATH", "ORB_SERVER_DB_PATH"); v != "" {
		s.DBPath = v
	}
	if v := firstNonEmptyEnv("SERVER_PUBLIC_URL", "ORB_SERVER_PUBLIC_URL"); v != "" {
		s.PublicURL = strings.TrimSuffix(v, "/")
	}
	if v := firstNonEmptyEnv("ORB_SERVER_ADMIN_TOKEN", "SERVER_ADMIN_TOKEN"); v != "" {
		s.AdminToken = v
	}
	if v := firstNonEmptyEnv("GITHUB_API_URL", "ORB_GITHUB_BASE_URL"); v != "" {
		s.GitHubBaseURL = v
	}
	if v := firstNonEmptyEnv("ORB_SERVER_POST_COMMENT", "ORB_POST_COMMENT"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			s.PostComment = b
		}
	}
	if v := os.Getenv("ORB_PROMPT_TEMPLATE"); strings.TrimSpace(v) != "" {
		s.PromptTemplate = v
	}
	if v := firstNonEmptyEnv("ORB_REPO_ROOT"); v != "" {
		s.RepoRoot = v
	}
	if url, ok := ollamaURLFromEnv(); ok {
		s.OllamaURL = url
	}
	if v := firstNonEmptyEnv("ORB_OLLAMA_MODEL", "OLLAMA_MODEL"); v != "" {
		s.OllamaModel = v
	}
	if v := firstNonEmptyEnv("ORB_OLLAMA_TEMPERATURE", "OLLAMA_TEMPERATURE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			s.OllamaTemp = f
		}
	}
	if v := firstNonEmptyEnv("ORB_OLLAMA_TOP_P", "OLLAMA_TOP_P"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			s.OllamaTopP = f
		}
	}
	if v := firstNonEmptyEnv("ORB_OLLAMA_MAX_TOKENS", "OLLAMA_MAX_TOKENS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			s.OllamaMaxTok = n
		}
	}
}

// ollamaURLFromEnv mirrors envx.OllamaURL but reports whether any related variable was set.
func ollamaURLFromEnv() (string, bool) {
	if url := strings.TrimSpace(os.Getenv("ORB_OLLAMA_URL")); url != "" {
		return url, true
	}
	if url := strings.TrimSpace(os.Getenv("OLLAMA_URL")); url != "" {
		return url, true
	}
	if host := strings.TrimSpace(os.Getenv("OLLAMA_HOST")); host != "" {
		return "http://" + host, true
	}
	return "", false
}

func (f *serverFile) applyTo(s *Server) {
	if f.Port != nil {
		if v := strings.TrimSpace(*f.Port); v != "" {
			s.Port = strings.TrimPrefix(v, ":")
		}
	}
	if f.PublicURL != nil {
		s.PublicURL = strings.TrimSuffix(strings.TrimSpace(*f.PublicURL), "/")
	}
	if f.DBPath != nil {
		if v := strings.TrimSpace(*f.DBPath); v != "" {
			s.DBPath = v
		}
	}
	if f.GitHubPAT != nil {
		if v := strings.TrimSpace(*f.GitHubPAT); v != "" {
			s.GitHubPAT = v
		}
	}
	if f.GitHubBaseURL != nil {
		s.GitHubBaseURL = strings.TrimSpace(*f.GitHubBaseURL)
	}
	if f.AdminToken != nil {
		s.AdminToken = strings.TrimSpace(*f.AdminToken)
	}
	if f.PromptTemplate != nil {
		s.PromptTemplate = strings.TrimSpace(*f.PromptTemplate)
	}
	if f.RepoRoot != nil {
		if v := strings.TrimSpace(*f.RepoRoot); v != "" {
			s.RepoRoot = v
		}
	}
	if f.PostComment != nil {
		s.PostComment = *f.PostComment
	}
	if f.OllamaURL != nil {
		if v := strings.TrimSpace(*f.OllamaURL); v != "" {
			s.OllamaURL = v
		}
	}
	if f.OllamaModel != nil {
		if v := strings.TrimSpace(*f.OllamaModel); v != "" {
			s.OllamaModel = v
		}
	}
	if f.OllamaTemp != nil {
		s.OllamaTemp = *f.OllamaTemp
	}
	if f.OllamaTopP != nil {
		s.OllamaTopP = *f.OllamaTopP
	}
	if f.OllamaMaxTok != nil {
		s.OllamaMaxTok = *f.OllamaMaxTok
	}
}

func parseServerFile(data []byte, format string) (serverFile, error) {
	var f serverFile
	var err error
	switch format {
	case "json":
		err = json.Unmarshal(data, &f)
	case "toml":
		err = toml.Unmarshal(data, &f)
	default:
		return serverFile{}, fmt.Errorf("unknown config format %q", format)
	}
	if err != nil {
		return serverFile{}, err
	}
	return f, nil
}

func detectConfigFormat(path string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return "json"
	case ".toml", ".tml":
		return "toml"
	}
	trim := bytes.TrimSpace(data)
	if len(trim) > 0 && trim[0] == '{' {
		return "json"
	}
	return "toml"
}

// LoadServer reads configuration: optional JSON/TOML file from configPath, then environment
// variables override any field when the corresponding env var is set (non-empty).
func LoadServer(configPath string) (Server, error) {
	s := serverDefaults()
	if strings.TrimSpace(configPath) != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return Server{}, fmt.Errorf("read config file: %w", err)
		}
		format := detectConfigFormat(configPath, data)
		f, err := parseServerFile(data, format)
		if err != nil {
			return Server{}, fmt.Errorf("parse config (%s): %w", format, err)
		}
		f.applyTo(&s)
	}
	applyServerEnvOverrides(&s)
	if err := validateServer(s); err != nil {
		return Server{}, err
	}
	return s, nil
}

func validateServer(s Server) error {
	if strings.TrimSpace(s.GitHubPAT) == "" {
		return fmt.Errorf("set GITHUB_PAT or GITHUB_TOKEN (repo + pull_requests access), or github_pat in config file")
	}
	return nil
}
