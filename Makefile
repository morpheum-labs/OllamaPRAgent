# Makefile for ollama-review-bot

GO := go
COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html
BIN_NAME := ollama-review-bot
BUILD_HARNESS := tests/build-harness.go
TEST_REPO_DIR := tests/repo
TEST_DIFF := $(TEST_REPO_DIR)/diff.patch
TEST_PR_BODY := $(TEST_REPO_DIR)/pr_body.txt
TEST_COMMITS := $(TEST_REPO_DIR)/commits.txt

.PHONY: build test test-harness run clean deep-clean

# Build the application
build:
	$(GO) build -o $(BIN_NAME) .

# Create test harness if needed
$(TEST_DIFF) $(TEST_PR_BODY) $(TEST_COMMITS): $(BUILD_HARNESS)
	$(GO) run $(BUILD_HARNESS)

# Create test harness target
test-harness: $(TEST_DIFF) $(TEST_PR_BODY) $(TEST_COMMITS)

# Run the application with test data
run: test-harness build
	./$(BIN_NAME) \
		--diff-file=$(TEST_DIFF) \
		--pr-body-file=$(TEST_PR_BODY) \
		--commits-file=$(TEST_COMMITS) \
		--repo-root=$(TEST_REPO_DIR) \
		--prompt-template=prompt.tmpl

# Run tests
test: test-harness
	$(GO) test -v ./...

# Run tests with coverage
coverage: test-harness
	$(GO) test -v ./... -coverprofile=$(COVERAGE_FILE)
	$(GO) tool cover -html=$(COVERAGE_FILE) -o=$(COVERAGE_HTML)
	@echo "Coverage report generated at $(COVERAGE_HTML)"

# Clean build artifacts and test output
clean:
	rm -f $(BIN_NAME) $(COVERAGE_FILE) $(COVERAGE_HTML)

# Deep clean (includes test harness)
deep-clean: clean
	rm -rf $(TEST_REPO_DIR)
