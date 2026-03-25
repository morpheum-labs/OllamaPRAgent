package gitprovider

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// FileProvider implements GitProvider interface for file-based operations
type FileProvider struct {
	DiffPath    string
	BodyPath    string
	CommitsPath string
	Repository  string // Optional repository name
	PRNumber    int    // Optional PR number
}

// NewFileProvider creates a new file-based provider
func NewFileProvider(diffPath, bodyPath, commitsPath string, repository string, prNumber int) *FileProvider {
	return &FileProvider{
		DiffPath:    diffPath,
		BodyPath:    bodyPath,
		CommitsPath: commitsPath,
		Repository:  repository,
		PRNumber:    prNumber,
	}
}

// GetPRContext reads files and returns a PRContext
func (f *FileProvider) GetPRContext() (*PRContext, error) {
	// Check if required files exist
	if f.DiffPath == "" || f.BodyPath == "" || f.CommitsPath == "" {
		return nil, errors.New("missing required file paths")
	}

	// Read diff file
	diffBytes, err := os.ReadFile(f.DiffPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read diff file: %w", err)
	}

	// Read PR body
	bodyBytes, err := os.ReadFile(f.BodyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read body file: %w", err)
	}

	// Read commits
	commitsBytes, err := os.ReadFile(f.CommitsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read commits file: %w", err)
	}

	// Parse commits into slice
	commitLines := strings.Split(strings.TrimSpace(string(commitsBytes)), "\n")

	// Return context
	return &PRContext{
		PRNumber:   f.PRNumber,
		Repository: f.Repository,
		// Note: We don't have head/base branch info from files
		Title:   "<placeholder>", // Not available from files directly
		Body:    string(bodyBytes),
		Commits: commitLines,
		Diff:    string(diffBytes),
	}, nil
}

// PostComment is a no-op for file provider
func (f *FileProvider) PostComment(comment string) error {
	// In file mode, we just print to stdout
	fmt.Println("=== Review Response ===")
	fmt.Println(comment)

	return nil
}
