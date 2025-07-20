package mmap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Security policies are hardcoded and always enforced:
// - Files must be owned by current user
// - Files must have exactly 0600 permissions
// - No directory restrictions (user can store anywhere)

// validateDataDirectory validates that a directory is safe for storing haystack data.
func validateDataDirectory(dir string) error {
	if dir == "" {
		return fmt.Errorf("data directory cannot be empty")
	}

	// Resolve to absolute path to prevent traversal
	_, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("invalid directory path: %w", err)
	}

	// Check for obvious path traversal attempts
	if strings.Contains(dir, "..") {
		return fmt.Errorf("path traversal not allowed in directory: %s", dir)
	}

	// Directory validation passed - user can store anywhere with proper file security

	// Get absolute path for directory creation
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Create directory if it doesn't exist, with secure permissions
	if err := os.MkdirAll(abs, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", abs, err)
	}

	// Validate directory permissions and ownership (always enforced)
	return validateDirectorySecurity(abs)
}

// buildSecureDataPath constructs a safe path for haystack data files.
func buildSecureDataPath(baseDir, filename string) (string, error) {
	if baseDir == "" {
		return "", fmt.Errorf("base directory cannot be empty")
	}

	// Ensure filename is safe (no path components)
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return "", fmt.Errorf("filename cannot contain path separators: %s", filename)
	}

	if strings.Contains(filename, "..") {
		return "", fmt.Errorf("filename cannot contain path traversal: %s", filename)
	}

	// Build secure path
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("invalid base directory: %w", err)
	}

	return filepath.Join(abs, filename), nil
}

// validateExistingFile validates security properties of an existing file.
func validateExistingFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	// Ensure it's a regular file
	if !info.Mode().IsRegular() {
		return fmt.Errorf("path %s is not a regular file", path)
	}

	// Always enforce strict permissions (opinionated security)
	if info.Mode().Perm() != 0600 {
		return fmt.Errorf("file %s must have 0600 permissions, got %o",
			path, info.Mode().Perm())
	}

	// Always check ownership (opinionated security)
	if err := validateFileOwnership(path, info); err != nil {
		return fmt.Errorf("ownership validation failed for %s: %w", path, err)
	}

	return nil
}

// validateDirectorySecurity checks directory permissions and ownership.
func validateDirectorySecurity(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("failed to stat directory %s: %w", dir, err)
	}

	// Ensure it's a directory
	if !info.IsDir() {
		return fmt.Errorf("path %s is not a directory", dir)
	}

	// Always check ownership (opinionated security)
	if err := validateFileOwnership(dir, info); err != nil {
		return fmt.Errorf("directory ownership validation failed for %s: %w", dir, err)
	}

	// Directory should not be world-writable
	if info.Mode().Perm()&0002 != 0 {
		return fmt.Errorf("directory %s is world-writable (security risk)", dir)
	}

	return nil
}

// validateFileOwnership ensures file is owned by current user.
func validateFileOwnership(path string, info os.FileInfo) error {
	currentUID := os.Getuid()

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("unable to get file ownership information for %s", path)
	}

	if int(stat.Uid) != currentUID {
		return fmt.Errorf("file %s must be owned by current user (UID %d), got UID %d",
			path, currentUID, stat.Uid)
	}

	return nil
}

// secureFileCreate creates a file with secure permissions and validates ownership.
func secureFileCreate(path string) (*os.File, error) {
	// Validate path is secure
	dir := filepath.Dir(path)
	if err := validateDataDirectory(dir); err != nil {
		return nil, fmt.Errorf("insecure directory for file %s: %w", path, err)
	}

	// Create file with secure permissions
	// #nosec G304 - Path validated by validateDataDirectory above
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create secure file %s: %w", path, err)
	}

	// Validate the created file meets security requirements
	if err := validateExistingFile(path); err != nil {
		var cleanupErrs []error
		if closeErr := file.Close(); closeErr != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("file.Close: %w", closeErr))
		}
		if removeErr := os.Remove(path); removeErr != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("os.Remove: %w", removeErr))
		}

		if len(cleanupErrs) > 0 {
			return nil, fmt.Errorf("created file failed security validation: %w (cleanup errors: %v)", err, cleanupErrs)
		}
		return nil, fmt.Errorf("created file failed security validation: %w", err)
	}

	return file, nil
}
