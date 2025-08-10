#!/usr/bin/env bash
#
# deploy.sh - Deploy Haystack to Fly.io using tree hash from HEAD
#
# This script:
# 1. Gets the tree hash from HEAD
# 2. Verifies the Docker image exists
# 3. Substitutes TREE_HASH in fly.toml
# 4. Deploys to Fly.io

set -euo pipefail

# Configuration
DOCKER_REPO="${DOCKER_REPO:-nomasters/haystack}"
DOCKER_REGISTRY="${DOCKER_REGISTRY:-docker.io}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Get tree hash from HEAD
get_tree_hash() {
    local repo_root
    repo_root=$(git rev-parse --show-toplevel)
    
    if [ -f "${repo_root}/scripts/tree-hash.sh" ]; then
        (cd "$repo_root" && ./scripts/tree-hash.sh)
    else
        log_error "Cannot find scripts/tree-hash.sh"
        exit 1
    fi
}

# Main deployment
main() {
    log_info "=== Haystack Fly.io Deployment ==="
    
    # Get the tree hash
    local tree_hash
    tree_hash=$(get_tree_hash)
    log_info "Tree hash: ${tree_hash}"
    
    # Build the image tag
    local image_tag="${DOCKER_REGISTRY}/${DOCKER_REPO}:tree-${tree_hash}"
    
    # Check if the Docker image exists
    log_info "Checking for image: ${image_tag}"
    if ! docker manifest inspect "${image_tag}" >/dev/null 2>&1; then
        log_error "Image not found: ${image_tag}"
        log_error "Run 'make docker-push' from repository root first"
        exit 1
    fi
    log_info "✓ Image found"
    
    # Substitute TREE_HASH in fly.toml
    log_info "Generating fly.toml with tree hash: ${tree_hash}"
    sed "s/TREE_HASH/${tree_hash}/g" fly.toml > fly.toml.generated
    
    # Show what we're deploying
    log_info "Deploying image: ${image_tag}"
    
    # Deploy to Fly.io
    if fly deploy --config fly.toml.generated; then
        log_info "✓ Deployment successful!"
        
        # Clean up generated file
        rm -f fly.toml.generated
        
        # Show app info
        fly status
    else
        log_error "Deployment failed!"
        rm -f fly.toml.generated
        exit 1
    fi
}

# Run main function
main "$@"