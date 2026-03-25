package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/morpheum-labs/OllamaPRAgent/internal/gitprovider"
)

func TestBuildPrompt(t *testing.T) {
	// Get current directory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Find the project root by going up from current directory
	projectRoot, err := findProjectRoot(wd)
	if err != nil {
		t.Fatalf("Failed to find project root: %v", err)
	}

	// Define paths relative to project root
	repoDir := filepath.Join(projectRoot, "tests", "repo")
	diffPath := filepath.Join(repoDir, "diff.patch")
	bodyPath := filepath.Join(repoDir, "pr_body.txt")
	commitsPath := filepath.Join(repoDir, "commits.txt")
	templatePath := filepath.Join(projectRoot, "internal", "prompt", "default_prompt.tmpl")

	// Create inputs
	diffContent, err := os.ReadFile(diffPath)
	if err != nil {
		t.Fatalf("Failed to read diff file: %v", err)
	}

	bodyContent, err := os.ReadFile(bodyPath)
	if err != nil {
		t.Fatalf("Failed to read PR body file: %v", err)
	}

	commitsContent, err := os.ReadFile(commitsPath)
	if err != nil {
		t.Fatalf("Failed to read commits file: %v", err)
	}

	prContext := &gitprovider.PRContext{
		Diff:    string(diffContent),
		Body:    string(bodyContent),
		Commits: strings.Split(strings.TrimSpace(string(commitsContent)), "\n"),
	}

	// Skip test if test repo files don't exist yet
	if _, err := os.Stat(diffPath); os.IsNotExist(err) {
		t.Skipf("Test repo files not found at %s. Run 'make test-harness' first.", diffPath)
	}

	t.Run("BuildPrompt with valid inputs", func(t *testing.T) {
		// Test the legacy method
		t.Run("Legacy BuildPrompt", func(t *testing.T) {

			// Build prompt
			result, err := BuildPromptFromContext(prContext, repoDir, templatePath)
			if err != nil {
				t.Fatalf("BuildPrompt failed: %v", err)
			}

			// Verify prompt contains expected sections
			requiredSections := []string{
				"## PR Description",
				"## Commit Messages",
				"## Diff",
				"```diff",
			}

			for _, section := range requiredSections {
				if !strings.Contains(result, section) {
					t.Errorf("Expected prompt to contain '%s', but it didn't", section)
				}
			}

			// Verify PR body was included
			if !strings.Contains(result, string(bodyContent)) {
				t.Error("Expected prompt to contain PR body content, but it didn't")
			}

			// Verify commits were included
			for _, commit := range strings.Split(strings.TrimSpace(string(commitsContent)), "\n") {
				if !strings.Contains(result, commit) {
					t.Errorf("Expected prompt to contain commit message '%s', but it didn't", commit)
				}
			}

			// Verify diff was included
			if !strings.Contains(result, string(diffContent)) {
				t.Error("Expected prompt to contain diff content, but it didn't")
			}
		})

		// Test the new context-based method
		t.Run("BuildPromptFromContext", func(t *testing.T) {
			// Create a file provider
			fileProvider := gitprovider.NewFileProvider(diffPath, bodyPath, commitsPath, "test/repo", 123)

			// Get PR context
			prContext, err := fileProvider.GetPRContext()
			if err != nil {
				t.Fatalf("Failed to get PR context: %v", err)
			}

			// Build prompt from context
			result, err := BuildPromptFromContext(prContext, repoDir, templatePath)
			if err != nil {
				t.Fatalf("BuildPromptFromContext failed: %v", err)
			}

			// Verify prompt contains expected sections
			requiredSections := []string{
				"## PR Description",
				"## Commit Messages",
				"## Diff",
				"```diff",
			}

			for _, section := range requiredSections {
				if !strings.Contains(result, section) {
					t.Errorf("Expected prompt to contain '%s', but it didn't", section)
				}
			}

			// Verify PR body was included
			bodyContent, err := os.ReadFile(bodyPath)
			if err != nil {
				t.Fatalf("Failed to read PR body file: %v", err)
			}
			if !strings.Contains(result, string(bodyContent)) {
				t.Error("Expected prompt to contain PR body content, but it didn't")
			}
		})
	})

	t.Run("BuildPrompt with invalid template", func(t *testing.T) {
		// Create a temporary invalid template file
		invalidTemplatePath := filepath.Join(t.TempDir(), "invalid.tmpl")
		if err := os.WriteFile(invalidTemplatePath, []byte("{{ .InvalidField }}"), 0644); err != nil {
			t.Fatalf("Failed to create invalid template file: %v", err)
		}

		// Create inputs with invalid template
		// Build prompt should not fail with template execution error
		_, err := BuildPromptFromContext(prContext, repoDir, invalidTemplatePath)
		if err == nil {
			t.Error("Expected error for invalid template, but got nil")
		}
	})

	t.Run("Template data structure is correctly populated", func(t *testing.T) {
		// Read input files
		// Create a custom template that outputs JSON-like structure for easier verification
		customTemplatePath := filepath.Join(t.TempDir(), "custom.tmpl")
		customTemplate := `PRBody:{{.PRBody}}
CommitsCount:{{len .Commits}}
DiffIncluded:{{if .Diff}}true{{else}}false{{end}}
ChangedFilesCount:{{len .ChangedFiles}}`

		if err := os.WriteFile(customTemplatePath, []byte(customTemplate), 0644); err != nil {
			t.Fatalf("Failed to create custom template file: %v", err)
		}

		// Build prompt with custom template
		result, err := BuildPromptFromContext(prContext, repoDir, customTemplatePath)
		if err != nil {
			t.Fatalf("BuildPrompt failed: %v", err)
		}

		// Verify template data was correctly populated
		if !strings.Contains(result, "PRBody:"+string(bodyContent)) {
			t.Error("Expected PRBody field to be populated correctly")
		}
		if !strings.Contains(result, "DiffIncluded:true") {
			t.Error("Expected Diff field to be populated")
		}

		// We know there should be at least one file in the diff
		if strings.Contains(result, "ChangedFilesCount:0") {
			t.Error("Expected ChangedFiles to contain at least one file")
		}
	})
}

func TestDetectLang(t *testing.T) {
	testCases := []struct {
		filename string
		expected string
	}{
		{"file.go", "go"},
		{"script.js", "javascript"},
		{"data.json", "json"},
		{"component.ts", "typescript"},
		{"script.py", "python"},
		{"lib.rs", "rust"},
		{"App.java", "java"},
		{"util.c", "c"},
		{"header.h", "c"},
		{"class.cpp", "cpp"},
		{"header.hpp", "cpp"},
		{"config.hcl", "hcl"},
		{"job.nomad", "hcl"},
		{"infra.tf", "hcl"},
		{"config.toml", "toml"},
		{"README.md", ""}, // No specific language for .md
		{"Makefile", ""},  // No specific language for Makefile
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			result := detectLang(tc.filename)
			if result != tc.expected {
				t.Errorf("detectLang(%q) = %q, want %q", tc.filename, result, tc.expected)
			}
		})
	}
}

// findProjectRoot finds the project root by looking for go.mod file
func findProjectRoot(dir string) (string, error) {
	for {
		// Check if go.mod exists in the current directory
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		// Move up one directory
		parentDir := filepath.Dir(dir)
		if parentDir == dir {
			// Reached the file system root without finding go.mod
			return "", fmt.Errorf("couldn't find project root (no go.mod found)")
		}
		dir = parentDir
	}
}

// TestTemplateParsing tests that template parsing works correctly
func TestTemplateParsing(t *testing.T) {
	testTemplate := `
Title: {{ .PRTitle }}
Body: {{ .PRBody }}
Commits: {{ len .Commits }}
Files: {{ len .ChangedFiles }}
`

	data := TemplateData{
		PRTitle: "Test PR",
		PRBody:  "Test body",
		Commits: []string{"Commit 1", "Commit 2"},
		Diff:    "test diff",
		ChangedFiles: []FileContent{
			{Path: "test.go", Content: "package main", Lang: "go"},
		},
	}

	tmpl, err := template.New("test").Parse(testTemplate)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("Template execution failed: %v", err)
	}

	result := buf.String()

	expectedParts := []string{
		"Title: Test PR",
		"Body: Test body",
		"Commits: 2",
		"Files: 1",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Expected template output to contain %q, but it didn't", part)
		}
	}
}
