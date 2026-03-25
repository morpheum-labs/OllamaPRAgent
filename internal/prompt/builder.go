package prompt

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/morpheum-labs/OllamaPRAgent/internal/gitprovider"
	"github.com/sourcegraph/go-diff/diff"
)

type Inputs struct {
	DiffPath    string
	BodyPath    string
	CommitsPath string
	RepoRoot    string
	Template    string
}

type FileContent struct {
	Path    string
	Content string
	Lang    string
}

type TemplateData struct {
	PRTitle      string
	PRBody       string
	Commits      []string
	Diff         string
	ChangedFiles []FileContent
}

//go:embed default_prompt.tmpl
var tmplBytes []byte

// BuildPromptFromContext builds a prompt using a PRContext
func BuildPromptFromContext(context *gitprovider.PRContext, repoRoot string, templatePath string) (string, error) {
	// Parse diff to extract changed file paths
	parsedDiffs, err := diff.ParseMultiFileDiff([]byte(context.Diff))
	if err != nil {
		return "", fmt.Errorf("failed to parse diff: %w", err)
	}

	filePaths := []string{}

	for _, f := range parsedDiffs {
		if f.NewName != "/dev/null" {
			filePaths = append(filePaths, strings.TrimPrefix(f.NewName, "b/"))
		}
	}

	// Read full file contents
	files := []FileContent{}

	for _, path := range filePaths {
		fullPath := filepath.Join(repoRoot, path)
		contents, err := os.ReadFile(fullPath)
		// TODO: Maybe skip some full files, eg. go.mod or go.sum, maybe make this configurable
		if err != nil {
			// Skip missing files (e.g. deleted)
			continue
		}

		files = append(files, FileContent{
			Path:    path,
			Content: string(contents),
			Lang:    detectLang(path),
		})
	}

	if templatePath != "" {
		tmplBytes, err = os.ReadFile(templatePath)
		if err != nil {
			return "", fmt.Errorf("failed to read template file: %w", err)
		}
	}

	tmpl, err := template.New("prompt").Parse(string(tmplBytes))
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	data := TemplateData{
		PRTitle:      context.Title,
		PRBody:       context.Body,
		Commits:      context.Commits,
		Diff:         context.Diff,
		ChangedFiles: files,
	}

	var buf bytes.Buffer

	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func detectLang(filename string) string {
	switch filepath.Ext(filename) {
	case ".go":
		return "go"
	case ".js":
		return "javascript"
	case ".json":
		return "json"
	case ".ts":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp":
		return "cpp"
	case ".hcl":
		return "hcl"
	case ".nomad":
		return "hcl"
	case ".tf":
		return "hcl"
	case ".toml":
		return "toml"
	default:
		return ""
	}
}
