package ollama

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Ping checks that the Ollama HTTP API responds at baseURL (e.g. http://localhost:11434).
func Ping(baseURL string) error {
	baseURL = strings.TrimSuffix(baseURL, "/")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(baseURL + "/api/tags")
	if err != nil {
		return fmt.Errorf("ollama unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("ollama returned %s: %s", resp.Status, string(body))
	}
	var out struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("ollama tags response: %w", err)
	}
	return nil
}

// ModelLoaded returns true if model appears in /api/tags (best-effort).
func ModelLoaded(baseURL, model string) bool {
	baseURL = strings.TrimSuffix(baseURL, "/")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(baseURL + "/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var out struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false
	}
	for _, m := range out.Models {
		if m.Name == model || strings.HasPrefix(m.Name, model+":") {
			return true
		}
	}
	return false
}
