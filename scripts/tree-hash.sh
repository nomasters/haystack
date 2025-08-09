#!/usr/bin/env bash
#
# tree-hash.sh - Calculate deterministic hash of source files
#
# This script generates a hash based on the content of files that affect
# the build output. It ensures we can detect when a rebuild is actually needed.

set -euo pipefail

# Files and directories that affect the build
# Only includes actual source code directories and build-critical files
BUILD_PATHS=(
    "cmd"
    "needle"
    "storage"
    "server"
    "client"
    "go.mod"
    "go.sum"
    "Dockerfile"
)

# Function to get git tree hash for specific paths
calculate_tree_hash() {
    # Use git ls-tree to get a deterministic hash of file contents
    # This is better than sha256sum because it's consistent across platforms
    
    # Get all files from our specific paths
    local files=""
    for path in "${BUILD_PATHS[@]}"; do
        if [ -e "$path" ]; then
            if [ -d "$path" ]; then
                # For directories, get all files within
                files="$files $(git ls-files "$path" 2>/dev/null || true)"
            else
                # For files, add directly
                files="$files $path"
            fi
        fi
    done
    
    # Remove duplicates and sort for deterministic output
    files=$(echo $files | tr ' ' '\n' | sort -u | tr '\n' ' ')
    
    if [ -z "$files" ]; then
        echo "Error: No source files found" >&2
        exit 1
    fi
    
    # Calculate hash using git's internal tree hashing
    # This gives us a content-based hash that's consistent
    echo $files | tr ' ' '\n' | xargs git hash-object | git hash-object --stdin
}

# Main execution
main() {
    # Ensure we're in a git repository
    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        echo "Error: Not in a git repository" >&2
        exit 1
    fi
    
    # Calculate and output the hash (first 12 characters for brevity)
    local full_hash=$(calculate_tree_hash)
    echo "${full_hash:0:12}"
}

# Run main function
main "$@"