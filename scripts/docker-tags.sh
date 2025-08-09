#!/usr/bin/env bash
#
# docker-tags.sh - Manage Docker image tags without rebuilding
#
# This script handles tag promotion and management for existing images

set -euo pipefail

# Load configuration from environment or use defaults
DOCKER_REGISTRY="${DOCKER_REGISTRY:-docker.io}"
DOCKER_REPO="${DOCKER_REPO:-nomasters/haystack}"

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

# Get the current git commit SHA (short)
get_commit_sha() {
    git rev-parse --short HEAD
}

# Check if a Docker image exists
image_exists() {
    local image="$1"
    
    if docker manifest inspect "$image" >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Promote an image with additional tags using buildx imagetools
promote_image() {
    local source_ref="$1"
    shift
    local new_tags=("$@")
    
    # Ensure source exists
    if ! image_exists "$source_ref"; then
        log_error "Source image does not exist: $source_ref"
        exit 1
    fi
    
    log_info "Promoting image: $source_ref"
    
    # Build the tag arguments for imagetools create
    local tag_args=()
    for tag in "${new_tags[@]}"; do
        local full_tag="${DOCKER_REGISTRY}/${DOCKER_REPO}:${tag}"
        tag_args+=("--tag" "$full_tag")
        log_info "  Adding tag: $tag"
    done
    
    # Use buildx imagetools to create new tags without rebuilding
    # This works across platforms and registries
    docker buildx imagetools create \
        "${tag_args[@]}" \
        "$source_ref"
    
    log_info "Promotion complete!"
}

# List all tags for the repository
list_tags() {
    log_info "Fetching tags for ${DOCKER_REPO}..."
    
    # This works for Docker Hub
    # For other registries, might need different approach
    if [ "$DOCKER_REGISTRY" = "docker.io" ]; then
        curl -s "https://hub.docker.com/v2/repositories/${DOCKER_REPO}/tags?page_size=100" | \
            jq -r '.results[].name' | \
            sort
    else
        log_warn "Tag listing not implemented for registry: $DOCKER_REGISTRY"
        log_info "Use: docker search ${DOCKER_REGISTRY}/${DOCKER_REPO}"
    fi
}

# Find image by commit SHA
find_image_by_commit() {
    local commit_sha="$1"
    local sha_tag="${DOCKER_REGISTRY}/${DOCKER_REPO}:sha-${commit_sha}"
    
    if image_exists "$sha_tag"; then
        echo "$sha_tag"
        return 0
    else
        return 1
    fi
}

# Main command dispatcher
main() {
    local command="${1:-}"
    shift || true
    
    case "$command" in
        promote)
            if [ $# -lt 2 ]; then
                log_error "Usage: $0 promote <source-ref> <new-tag> [<new-tag>...]"
                exit 1
            fi
            promote_image "$@"
            ;;
            
        release)
            # Special case for releases - find image by current commit
            if [ $# -lt 1 ]; then
                log_error "Usage: $0 release <version-tag> [<additional-tags>...]"
                exit 1
            fi
            
            local commit_sha
            commit_sha=$(get_commit_sha)
            local source_image
            
            if source_image=$(find_image_by_commit "$commit_sha"); then
                log_info "Found image for commit $commit_sha: $source_image"
                promote_image "$source_image" "$@"
            else
                log_error "No image found for commit SHA: $commit_sha"
                log_error "Build the image first with: make docker-build"
                exit 1
            fi
            ;;
            
        list)
            list_tags
            ;;
            
        exists)
            if [ $# -lt 1 ]; then
                log_error "Usage: $0 exists <tag>"
                exit 1
            fi
            
            local check_tag="${DOCKER_REGISTRY}/${DOCKER_REPO}:$1"
            if image_exists "$check_tag"; then
                log_info "Image exists: $check_tag"
                exit 0
            else
                log_info "Image does not exist: $check_tag"
                exit 1
            fi
            ;;
            
        *)
            cat <<EOF
Usage: $0 <command> [arguments]

Commands:
    promote <source-ref> <new-tag> [<new-tag>...]
        Add new tags to an existing image
        
    release <version-tag> [<additional-tags>...]
        Promote the current commit's image to a release
        
    list
        List all available tags
        
    exists <tag>
        Check if a tag exists

Environment variables:
    DOCKER_REGISTRY  Registry to use (default: docker.io)
    DOCKER_REPO      Repository name (default: nomasters/haystack)

Examples:
    $0 promote sha-abc123 v1.0.0 latest
    $0 release v1.0.0 latest
    $0 exists v1.0.0
EOF
            exit 1
            ;;
    esac
}

# Run main function
main "$@"