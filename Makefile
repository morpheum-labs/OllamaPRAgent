# Makefile for OllamaPRAgent (ollama-review-bot)

GO := go
MODULE := github.com/morpheum-labs/OllamaPRAgent
BIN_NAME := ollama-review-bot
DIST_DIR := dist

COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html
BUILD_HARNESS := tests/build-harness.go
TEST_REPO_DIR := tests/repo
TEST_DIFF := $(TEST_REPO_DIR)/diff.patch
TEST_PR_BODY := $(TEST_REPO_DIR)/pr_body.txt
TEST_COMMITS := $(TEST_REPO_DIR)/commits.txt
DEFAULT_PROMPT := internal/prompt/default_prompt.tmpl

# Injected into the binary; for release builds use the same value as your git tag:
#   make dist VERSION=v1.2.3
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.0.0-dev")

# Git annotated tag (must be passed explicitly): make tag TAG=v1.2.3
TAG ?=
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

GO_LDFLAGS := -s -w \
	-X 'main.version=$(VERSION)' \
	-X 'main.commit=$(GIT_COMMIT)' \
	-X 'main.buildTime=$(BUILD_TIME)'

# Cross-compile targets for dist (GOOS/GOARCH)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: build build-release run test test-harness coverage clean clean-dist deep-clean \
	version print-version install dist tag push-tag publish release release-check lint

# --- version ---

version print-version:
	@echo "$(VERSION)"

# --- build ---

build:
	$(GO) build -trimpath -ldflags "$(GO_LDFLAGS)" -o $(BIN_NAME) .

# Release-style local binary (same as build; name makes CI/docs intent clear)
build-release: build

install:
	$(GO) install -trimpath -ldflags "$(GO_LDFLAGS)" .

# --- run ---

$(TEST_DIFF) $(TEST_PR_BODY) $(TEST_COMMITS): $(BUILD_HARNESS)
	$(GO) run $(BUILD_HARNESS)

test-harness: $(TEST_DIFF) $(TEST_PR_BODY) $(TEST_COMMITS)

run: test-harness build
	./$(BIN_NAME) \
		--provider=file \
		--file-diff=$(TEST_DIFF) \
		--file-pr-body=$(TEST_PR_BODY) \
		--file-commits=$(TEST_COMMITS) \
		--repo-root=$(TEST_REPO_DIR) \
		--prompt-template=$(DEFAULT_PROMPT)

# --- test ---

test: test-harness
	$(GO) test -v ./...

coverage: test-harness
	$(GO) test -v ./... -coverprofile=$(COVERAGE_FILE)
	$(GO) tool cover -html=$(COVERAGE_FILE) -o=$(COVERAGE_HTML)
	@echo "Coverage report generated at $(COVERAGE_HTML)"

lint:
	$(GO) vet ./...
	golangci-lint run ./...

# --- artifacts ---

clean-dist:
	rm -rf $(DIST_DIR)

dist: clean-dist
	@mkdir -p $(DIST_DIR)
	@set -e; for p in $(PLATFORMS); do \
		goos=$${p%%/*}; \
		goarch=$${p##*/}; \
		ext=; \
		[ "$$goos" = windows ] && ext=.exe || true; \
		out="$(DIST_DIR)/$(BIN_NAME)-$$goos-$$goarch$$ext"; \
		echo "build $$out"; \
		GOOS=$$goos GOARCH=$$goarch $(GO) build -trimpath -ldflags "$(GO_LDFLAGS)" -o "$$out" .; \
	done
	@shasum -a 256 $(DIST_DIR)/* > $(DIST_DIR)/SHA256SUMS 2>/dev/null || sha256sum $(DIST_DIR)/* > $(DIST_DIR)/SHA256SUMS
	@echo "Artifacts in $(DIST_DIR)/ (checksums: $(DIST_DIR)/SHA256SUMS)"

# --- tag / publish ---

# Usage: make tag TAG=v1.2.3
tag:
	@if [ -z "$(TAG)" ]; then echo "usage: make tag TAG=v1.2.3"; exit 1; fi
	@echo "$(TAG)" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+' || (echo "TAG must be vMAJOR.MINOR.PATCH (e.g. v1.2.3)" && exit 1)
	git tag -a "$(TAG)" -m "Release $(TAG)"

# Usage: make push-tag TAG=v1.2.3  (alias: publish)
push-tag publish:
	@if [ -z "$(TAG)" ]; then echo "usage: make push-tag TAG=v1.2.3"; exit 1; fi
	git push origin "$(TAG)"

release-check: test
	@test -z "$$(git status --porcelain)" || (echo "Working tree is not clean; commit or stash before release." && exit 1)

# Tests, checksumed multi-platform binaries, then remind next steps.
# Typical: VERSION=v1.2.3 make dist && make release-check && make tag TAG=v1.2.3 && make publish TAG=v1.2.3
release: release-check dist
	@echo ""
	@echo "Next: make tag TAG=vX.Y.Z && make publish TAG=vX.Y.Z"
	@echo "Optional: gh release create \"\$$TAG\" dist/* --generate-notes"

# --- clean ---

clean: clean-dist
	rm -f $(BIN_NAME) $(COVERAGE_FILE) $(COVERAGE_HTML)

deep-clean: clean
	rm -rf $(TEST_REPO_DIR)
