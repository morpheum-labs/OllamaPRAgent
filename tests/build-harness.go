package main

// This file should act like a script to create a test harness for prompt building logic.
// It should:
// 	1. Shallow clone a repo at a particular sha
// 	2. Create a diff of the top commit
//  3. Write the top commit message to a commits file
//
// These generated files should be in `tests/repo` so that tests can be run

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	repoURL = "https://git.iamthefij.com/iamthefij/yk-cli"
	gitSHA  = "34e4b130df92429513a05fc628607a5b81c0029a"
)

func main() {
	// Create test repository directory
	repoDir := filepath.Join("tests", "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		log.Fatalf("Failed to create test repo directory: %v", err)
	}

	// Clean up any existing repository
	if err := os.RemoveAll(filepath.Join(repoDir, ".git")); err != nil {
		log.Fatalf("Failed to clean up existing repository: %v", err)
	}

	// Initialize git repository
	if err := runCommand(repoDir, "git", "init"); err != nil {
		log.Fatalf("Failed to initialize git repository: %v", err)
	}

	// Add remote
	if err := runCommand(repoDir, "git", "remote", "add", "origin", repoURL); err != nil {
		log.Fatalf("Failed to add remote: %v", err)
	}

	// Fetch the specific commit
	if err := runCommand(repoDir, "git", "fetch", "--depth=2", "origin", gitSHA); err != nil {
		log.Fatalf("Failed to fetch commit: %v", err)
	}

	// Checkout the specific commit
	if err := runCommand(repoDir, "git", "checkout", gitSHA); err != nil {
		log.Fatalf("Failed to checkout commit: %v", err)
	}

	// Create a diff of the top commit
	diffOutput, err := execCommand(repoDir, "git", "show", "--patch", gitSHA)
	if err != nil {
		log.Fatalf("Failed to create diff: %v", err)
	}

	// Write diff to file
	diffPath := filepath.Join(repoDir, "diff.patch")
	if err := os.WriteFile(diffPath, []byte(diffOutput), 0644); err != nil {
		log.Fatalf("Failed to write diff file: %v", err)
	}
	fmt.Printf("Diff file created at: %s\n", diffPath)

	// Get commit message
	commitMsg, err := execCommand(repoDir, "git", "log", "-1", "--pretty=format:%B", gitSHA)
	if err != nil {
		log.Fatalf("Failed to get commit message: %v", err)
	}

	// Write commit message to file
	commitsPath := filepath.Join(repoDir, "commits.txt")
	if err := os.WriteFile(commitsPath, []byte(commitMsg), 0644); err != nil {
		log.Fatalf("Failed to write commits file: %v", err)
	}
	fmt.Printf("Commits file created at: %s\n", commitsPath)

	// Create empty PR body file for testing
	prBodyPath := filepath.Join(repoDir, "pr_body.txt")
	if err := os.WriteFile(prBodyPath, []byte("Test PR Body"), 0644); err != nil {
		log.Fatalf("Failed to write PR body file: %v", err)
	}
	fmt.Printf("PR body file created at: %s\n", prBodyPath)

	fmt.Println("Test harness setup complete!")
}

// runCommand executes a command in the specified directory and returns any error
func runCommand(dir string, command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// execCommand executes a command and returns its output
func execCommand(dir string, command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return string(output), err
}
