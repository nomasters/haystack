.PHONY: coverage test inspect install sec-scan lint fmt check
.PHONY: tree-hash docker-build docker-push docker-exists docker-promote docker-release docker-list

# Docker configuration with defaults (override with environment variables)
export DOCKER_REGISTRY ?= docker.io
export DOCKER_REPO ?= nomasters/haystack
export DOCKER_PLATFORMS ?= linux/amd64,linux/arm64,linux/arm/v7
export DOCKER_PUSH ?= false
export SKIP_GIT_CHECK ?= false

# Go targets
test:
	go test -v ./...

coverage:
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...

inspect: coverage
	go tool cover -html=coverage.txt

sec-scan:
	gosec -fmt=json -out=gosec-report.json -stdout -verbose=text ./...

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

check: fmt lint test
	@echo "All checks passed!"

update-deps:
	go get -u && go mod tidy

install:
	go install github.com/nomasters/haystack/cmd/haystack

# Docker targets - Local-first hermetic builds
# These commands work identically on your machine and in CI

# Calculate the tree hash of source files
tree-hash:
	@./scripts/tree-hash.sh

# Build Docker image (idempotent - only builds if tree hash changed)
# Requires clean git working directory (override with SKIP_GIT_CHECK=true)
docker-build:
	@./scripts/docker-build.sh

# Build and push Docker image to registry
docker-push:
	@DOCKER_PUSH=true ./scripts/docker-build.sh

# Check if image exists for current tree hash
docker-exists:
	@TREE_HASH=$$(./scripts/tree-hash.sh) && \
		./scripts/docker-tags.sh exists "tree-$$TREE_HASH"

# Promote current commit's image with additional tags
# Usage: make docker-promote TAGS="v1.0.0"
docker-promote:
	@if [ -z "$(TAGS)" ]; then \
		echo "Error: TAGS variable required. Usage: make docker-promote TAGS=\"v1.0.0\""; \
		exit 1; \
	fi
	@./scripts/docker-tags.sh release $(TAGS)

# Release workflow - build if needed, then tag
# Usage: make docker-release VERSION=v1.0.0
docker-release: docker-push
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION variable required. Usage: make docker-release VERSION=v1.0.0"; \
		exit 1; \
	fi
	@./scripts/docker-tags.sh release $(VERSION)

# List all Docker tags in the registry
docker-list:
	@./scripts/docker-tags.sh list

# Clean up local Docker buildx builders
docker-clean:
	@docker buildx rm haystack-builder 2>/dev/null || true

# Show current build information
docker-info:
	@echo "Tree hash:   $$(./scripts/tree-hash.sh)"
	@echo "Commit SHA:  $$(git rev-parse --short HEAD)"
	@echo "Branch:      $$(git rev-parse --abbrev-ref HEAD)"
	@echo "Registry:    $${DOCKER_REGISTRY:-docker.io}"
	@echo "Repository:  $${DOCKER_REPO:-nomasters/haystack}"