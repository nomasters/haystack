package mmap

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestDir creates a temporary directory for testing
func setupTestDir(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "haystack-security-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	})
	return tmpDir
}

func TestValidateDataDirectory(t *testing.T) {
	tmpDir := setupTestDir(t)
	otherDir := setupTestDir(t)

	tests := []struct {
		name      string
		dir       string
		wantError bool
		reason    string
	}{
		{
			name:      "empty directory",
			dir:       "",
			wantError: true,
			reason:    "empty directory path should be rejected",
		},
		{
			name:      "valid existing directory",
			dir:       tmpDir,
			wantError: false,
			reason:    "valid directory should be accepted",
		},
		{
			name:      "valid other directory",
			dir:       otherDir,
			wantError: false,
			reason:    "any valid directory should be accepted",
		},
		{
			name:      "path traversal with dots",
			dir:       "..",
			wantError: true,
			reason:    "path traversal should be rejected",
		},
		{
			name:      "path traversal with directory",
			dir:       tmpDir + "/../..",
			wantError: true,
			reason:    "path traversal in existing directory should be rejected",
		},
		{
			name:      "path traversal relative",
			dir:       "../",
			wantError: true,
			reason:    "relative path traversal should be rejected",
		},
		{
			name:      "path traversal complex",
			dir:       "dir/../..",
			wantError: true,
			reason:    "complex path traversal should be rejected",
		},
		{
			name:      "path traversal with target",
			dir:       "./../../etc",
			wantError: true,
			reason:    "path traversal to system directories should be rejected",
		},
		{
			name:      "extremely long path",
			dir:       string(make([]byte, 10000)), // Path longer than most filesystem limits
			wantError: true,
			reason:    "extremely long paths should be rejected",
		},
		{
			name:      "path with null bytes",
			dir:       "test\x00path",
			wantError: true,
			reason:    "path with null bytes should be rejected",
		},
		{
			name:      "path with invalid utf8",
			dir:       "test\xff\xfe\xfdpath",
			wantError: true,
			reason:    "path with invalid UTF-8 should be rejected",
		},
		{
			name:      "path with only null bytes",
			dir:       "\x00\x00\x00",
			wantError: true,
			reason:    "path with only null bytes should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDataDirectory(tt.dir)
			if (err != nil) != tt.wantError {
				t.Errorf("validateDataDirectory(%q) error = %v, wantError %v - %s", tt.dir, err, tt.wantError, tt.reason)
			}
		})
	}
}

// TestValidateDataDirectory_FilepathAbsError attempts to test the filepath.Abs() error path.
// Note: This is defensive code that's very difficult to trigger in practice on modern systems.
// filepath.Abs() almost never fails, but the error handling is there for completeness.
func TestValidateDataDirectory_FilepathAbsError(t *testing.T) {
	// Test case that can trigger filesystem-level errors which might hit filepath.Abs()
	t.Run("extremely_long_path", func(t *testing.T) {
		// Create an extremely long path that might exceed system limits
		longPath := string(make([]rune, 32768))
		err := validateDataDirectory(longPath)
		if err == nil {
			t.Error("Expected error for extremely long path")
		} else {
			t.Logf("Long path failed as expected: %v", err)
			// This error could come from filepath.Abs() or directory creation
			// Both are acceptable security boundaries
		}
	})

	// Test cases that might bypass path traversal check but could cause other errors
	pathsWithoutTraversal := []struct {
		name string
		path string
	}{
		{"control_characters", "test\x01\x02\x03path"},
		{"pipe_characters", "test|<>path"},
		{"unicode_control", "test\u0000\u0001path"},
	}

	for _, tc := range pathsWithoutTraversal {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDataDirectory(tc.path)
			// These might or might not fail depending on the system
			// The important thing is that IF they pass validation, they should be safe
			if err != nil {
				t.Logf("Path %q failed validation as expected: %v", tc.path, err)
			} else {
				t.Logf("Path %q passed validation (this may be system-dependent)", tc.path)
			}
		})
	}
	
	// Note: The filepath.Abs() error path (lines 24-26 in security.go) is defensive
	// programming that's extremely difficult to trigger in unit tests across platforms.
	// This is acceptable - some defensive error handling cannot be easily unit tested.
}

func TestBuildSecureDataPath(t *testing.T) {
	tmpDir := setupTestDir(t)

	tests := []struct {
		name      string
		baseDir   string
		filename  string
		wantError bool
		reason    string
	}{
		{
			name:      "empty base directory",
			baseDir:   "",
			filename:  "test.data",
			wantError: true,
			reason:    "empty base directory should be rejected",
		},
		{
			name:      "valid path construction",
			baseDir:   tmpDir,
			filename:  "test.data",
			wantError: false,
			reason:    "valid inputs should succeed",
		},
		{
			name:      "filename with unix path separator",
			baseDir:   tmpDir,
			filename:  "subdir/evil.data",
			wantError: true,
			reason:    "filename with unix path separators should be rejected",
		},
		{
			name:      "filename with windows path separator",
			baseDir:   tmpDir,
			filename:  "subdir\\evil.data",
			wantError: true,
			reason:    "filename with windows path separators should be rejected",
		},
		{
			name:      "filename with path traversal",
			baseDir:   tmpDir,
			filename:  "../evil.data",
			wantError: true,
			reason:    "filename with path traversal should be rejected",
		},
		{
			name:      "filename with complex path traversal",
			baseDir:   tmpDir,
			filename:  "../../etc/passwd",
			wantError: true,
			reason:    "filename with complex path traversal should be rejected",
		},
		{
			name:      "filename with dots only",
			baseDir:   tmpDir,
			filename:  "..",
			wantError: true,
			reason:    "filename with dots only should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := buildSecureDataPath(tt.baseDir, tt.filename)
			if (err != nil) != tt.wantError {
				t.Errorf("buildSecureDataPath(%q, %q) error = %v, wantError %v - %s", tt.baseDir, tt.filename, err, tt.wantError, tt.reason)
			}
			if !tt.wantError && err == nil {
				expected := filepath.Join(tt.baseDir, tt.filename)
				if path != expected {
					t.Errorf("buildSecureDataPath(%q, %q) = %q, want %q", tt.baseDir, tt.filename, path, expected)
				}
			}
		})
	}
}

func TestValidateExistingFile(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create test files with different configurations
	validFile := filepath.Join(tmpDir, "valid.data")
	wrongPermsFile := filepath.Join(tmpDir, "wrong-perms.data")
	
	// Create valid file
	f, err := os.Create(validFile)
	if err != nil {
		t.Fatalf("Failed to create valid test file: %v", err)
	}
	f.Close()
	if err := os.Chmod(validFile, 0600); err != nil {
		t.Fatalf("Failed to set correct permissions: %v", err)
	}

	// Create file with wrong permissions
	f, err = os.Create(wrongPermsFile)
	if err != nil {
		t.Fatalf("Failed to create wrong perms test file: %v", err)
	}
	f.Close()
	if err := os.Chmod(wrongPermsFile, 0644); err != nil {
		t.Fatalf("Failed to set wrong permissions: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		wantError bool
		reason    string
	}{
		{
			name:      "non-existent file",
			path:      "/nonexistent/file.data",
			wantError: true,
			reason:    "non-existent file should fail stat",
		},
		{
			name:      "directory instead of file",
			path:      tmpDir,
			wantError: true,
			reason:    "directory should fail regular file check",
		},
		{
			name:      "valid file with correct permissions",
			path:      validFile,
			wantError: false,
			reason:    "valid file should pass all checks",
		},
		{
			name:      "file with wrong permissions",
			path:      wrongPermsFile,
			wantError: true,
			reason:    "file without 0600 permissions should fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExistingFile(tt.path)
			if (err != nil) != tt.wantError {
				t.Errorf("validateExistingFile(%q) error = %v, wantError %v - %s", tt.path, err, tt.wantError, tt.reason)
			}
		})
	}
}

func TestValidateDirectorySecurity(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create a regular file for testing
	regularFile := filepath.Join(tmpDir, "not-a-directory.txt")
	f, err := os.Create(regularFile)
	if err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}
	f.Close()

	// Create a world-writable directory
	worldWritableDir := filepath.Join(tmpDir, "world-writable")
	if err := os.Mkdir(worldWritableDir, 0755); err != nil {
		t.Fatalf("Failed to create world-writable test dir: %v", err)
	}
	if err := os.Chmod(worldWritableDir, 0755|0002); err != nil {
		t.Fatalf("Failed to set world-writable permissions: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		wantError bool
		reason    string
	}{
		{
			name:      "non-existent directory",
			path:      "/nonexistent/directory",
			wantError: true,
			reason:    "non-existent directory should fail stat",
		},
		{
			name:      "regular file instead of directory",
			path:      regularFile,
			wantError: true,
			reason:    "regular file should fail directory check",
		},
		{
			name:      "valid directory",
			path:      tmpDir,
			wantError: false,
			reason:    "valid directory should pass all checks",
		},
		{
			name:      "world-writable directory",
			path:      worldWritableDir,
			wantError: true,
			reason:    "world-writable directory should fail security check",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDirectorySecurity(tt.path)
			if (err != nil) != tt.wantError {
				t.Errorf("validateDirectorySecurity(%q) error = %v, wantError %v - %s", tt.path, err, tt.wantError, tt.reason)
			}
		})
	}
}

func TestSecureFileCreate(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create a file with wrong permissions for failure test
	existingWrongPermsFile := filepath.Join(tmpDir, "existing-wrong-perms.data")
	f, err := os.Create(existingWrongPermsFile)
	if err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}
	f.Close()
	if err := os.Chmod(existingWrongPermsFile, 0644); err != nil {
		t.Fatalf("Failed to set wrong permissions: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		wantError bool
		reason    string
		cleanup   bool
	}{
		{
			name:      "invalid directory path",
			path:      "/nonexistent/directory/file.data",
			wantError: true,
			reason:    "file in non-existent directory should fail",
		},
		{
			name:      "valid new file creation",
			path:      filepath.Join(tmpDir, "new-secure.data"),
			wantError: false,
			reason:    "new file in valid directory should succeed",
			cleanup:   true,
		},
		{
			name:      "existing file with wrong permissions",
			path:      existingWrongPermsFile,
			wantError: true,
			reason:    "existing file with wrong permissions should fail validation and cleanup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := secureFileCreate(tt.path)
			if (err != nil) != tt.wantError {
				t.Errorf("secureFileCreate(%q) error = %v, wantError %v - %s", tt.path, err, tt.wantError, tt.reason)
			}
			
			if err == nil && file != nil {
				// Verify the file has correct permissions
				info, statErr := file.Stat()
				if statErr != nil {
					t.Errorf("Failed to stat created file: %v", statErr)
				} else if info.Mode().Perm() != 0600 {
					t.Errorf("Created file has wrong permissions: got %o, want 0600", info.Mode().Perm())
				}
				
				if closeErr := file.Close(); closeErr != nil {
					t.Errorf("Failed to close file: %v", closeErr)
				}
				
				if tt.cleanup {
					if removeErr := os.Remove(tt.path); removeErr != nil {
						t.Errorf("Failed to remove test file: %v", removeErr)
					}
				}
			}
		})
	}
}

// Keep the integration tests for higher-level functionality
func TestSecureStore(t *testing.T) {
	tmpDir := setupTestDir(t)

	t.Run("DefaultConfig", func(t *testing.T) {
		config := DefaultConfig()
		if config.DataDirectory != "." {
			t.Errorf("Expected DataDirectory '.', got %s", config.DataDirectory)
		}
	})

	t.Run("SecureDataFileCreation", func(t *testing.T) {
		dataPath := filepath.Join(tmpDir, "haystack.data")
		dataFile, err := newSecureDataFile(dataPath, 1000, 1024*1024)
		if err != nil {
			t.Errorf("NewSecureDataFile failed: %v", err)
			return
		}
		defer func() {
			if dataFile != nil {
				if err := dataFile.Close(); err != nil {
					t.Errorf("Failed to close data file: %v", err)
				}
			}
		}()

		if err := validateExistingFile(dataPath); err != nil {
			t.Errorf("Secure data file failed validation: %v", err)
		}
	})

	t.Run("SecureIndexCreation", func(t *testing.T) {
		indexPath := filepath.Join(tmpDir, "haystack.index")
		index, err := newSecureIndex(indexPath, 1000)
		if err != nil {
			t.Errorf("NewSecureIndex failed: %v", err)
			return
		}
		defer func() {
			if index != nil {
				if err := index.Close(); err != nil {
					t.Errorf("Failed to close index: %v", err)
				}
			}
		}()

		if err := validateExistingFile(indexPath); err != nil {
			t.Errorf("Secure index file failed validation: %v", err)
		}
	})
}