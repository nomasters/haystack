#!/usr/bin/env bash
#
# docker-build.sh - Hermetic Docker build with content-based tagging
#
# This script implements idempotent Docker builds that:
# 1. Check if an image with the current tree hash already exists
# 2. Build only if necessary
# 3. Tag with both tree hash and commit SHA

set -euo pipefail

# Load configuration from environment or use defaults
DOCKER_REGISTRY="${DOCKER_REGISTRY:-docker.io}"
DOCKER_REPO="${DOCKER_REPO:-nomasters/haystack}"
DOCKER_PLATFORMS="${DOCKER_PLATFORMS:-linux/amd64,linux/arm64}"
DOCKER_PUSH="${DOCKER_PUSH:-false}"
SKIP_GIT_CHECK="${SKIP_GIT_CHECK:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Get the current tree hash
get_tree_hash() {
    local script_dir
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    "${script_dir}/tree-hash.sh"
}

# Get the current git commit SHA (short)
get_commit_sha() {
    git rev-parse --short HEAD
}

# Get the current branch name
get_branch_name() {
    git rev-parse --abbrev-ref HEAD
}

# Check if a Docker image exists in the registry
image_exists() {
    local image="$1"
    
    # Try to inspect the manifest
    if docker manifest inspect "$image" >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Build multi-platform Docker image
build_image() {
    local tree_hash="$1"
    local commit_sha="$2"
    local branch="$3"
    
    local tree_tag="${DOCKER_REGISTRY}/${DOCKER_REPO}:tree-${tree_hash}"
    local sha_tag="${DOCKER_REGISTRY}/${DOCKER_REPO}:sha-${commit_sha}"
    local branch_tag="${DOCKER_REGISTRY}/${DOCKER_REPO}:${branch}"
    
    log_info "Building multi-platform image..."
    log_info "Platforms: ${DOCKER_PLATFORMS}"
    log_info "Tags: tree-${tree_hash}, sha-${commit_sha}, ${branch}"
    
    # Ensure buildx is available
    if ! docker buildx version >/dev/null 2>&1; then
        log_error "Docker buildx is not available. Please install Docker with buildx support."
        exit 1
    fi
    
    # Ensure clean buildx state - always remove and recreate for consistency
    # This avoids "existing instance" errors
    if docker buildx ls | grep -q "haystack-builder"; then
        log_info "Removing existing buildx builder..."
        docker buildx rm haystack-builder 2>/dev/null || true
    fi
    
    log_info "Creating buildx builder..."
    docker buildx create --name haystack-builder --use --bootstrap
    
    # Build the image with all tags
    local build_args=(
        "--platform=${DOCKER_PLATFORMS}"
        "--tag=${tree_tag}"
        "--tag=${sha_tag}"
        "--tag=${branch_tag}"
    )
    
    # Add push flag if requested
    if [ "${DOCKER_PUSH}" = "true" ]; then
        build_args+=("--push")
    else
        build_args+=("--load")
        log_warn "Building for local platform only (--load mode). Set DOCKER_PUSH=true for multi-platform push."
        build_args=("--tag=${tree_tag}" "--tag=${sha_tag}" "--tag=${branch_tag}")
    fi
    
    docker buildx build "${build_args[@]}" .
    
    log_info "Build complete!"
}

# Add additional tags to an existing image
add_tags() {
    local source_tag="$1"
    local commit_sha="$2"
    local branch="$3"
    
    local sha_tag="${DOCKER_REGISTRY}/${DOCKER_REPO}:sha-${commit_sha}"
    local branch_tag="${DOCKER_REGISTRY}/${DOCKER_REPO}:${branch}"
    
    log_info "Image already exists with tree hash, adding new tags..."
    
    if [ "${DOCKER_PUSH}" = "true" ]; then
        # Pull the existing image
        docker pull "$source_tag"
        
        # Tag with commit SHA
        docker tag "$source_tag" "$sha_tag"
        docker push "$sha_tag"
        
        # Tag with branch name
        docker tag "$source_tag" "$branch_tag"
        docker push "$branch_tag"
        
        log_info "Tags added and pushed: sha-${commit_sha}, ${branch}"
    else
        log_info "Skipping tag operations (DOCKER_PUSH=false)"
    fi
}

# Check if git working directory is clean
check_git_clean() {
    if ! git diff --quiet || ! git diff --cached --quiet; then
        log_error "Git working directory is not clean!"
        log_error "Please commit or stash your changes before building."
        log_info "Uncommitted changes:"
        git status --short
        return 1
    fi
    
    # Check for untracked files (excluding ignored files)
    if [ -n "$(git ls-files --others --exclude-standard)" ]; then
        log_warn "Untracked files detected:"
        git ls-files --others --exclude-standard
        log_error "Please add or ignore untracked files before building."
        return 1
    fi
    
    return 0
}

# Main execution
main() {
    # Check if git is clean (unless explicitly skipped)
    if [ "${SKIP_GIT_CHECK}" != "true" ]; then
        if ! check_git_clean; then
            log_error "Refusing to build with uncommitted changes."
            log_info "This ensures Docker images only contain committed code."
            log_info "To bypass for local testing: SKIP_GIT_CHECK=true make docker-build"
            exit 1
        fi
    else
        log_warn "Skipping git clean check (SKIP_GIT_CHECK=true)"
    fi
    
    # Get current hashes
    local tree_hash
    local commit_sha
    local branch
    
    tree_hash=$(get_tree_hash)
    commit_sha=$(get_commit_sha)
    branch=$(get_branch_name)
    
    log_info "Tree hash: ${tree_hash}"
    log_info "Commit SHA: ${commit_sha}"
    log_info "Branch: ${branch}"
    
    # Check if image with tree hash already exists
    local tree_tag="${DOCKER_REGISTRY}/${DOCKER_REPO}:tree-${tree_hash}"
    
    if image_exists "$tree_tag"; then
        log_info "Image already exists for tree hash: ${tree_hash}"
        add_tags "$tree_tag" "$commit_sha" "$branch"
    else
        log_info "No existing image for tree hash: ${tree_hash}"
        build_image "$tree_hash" "$commit_sha" "$branch"
    fi
    
    log_info "Done!"
}

# Run main function
main "$@"