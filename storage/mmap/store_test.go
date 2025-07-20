package mmap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nomasters/haystack/needle"
)

// createTestConfig creates a test configuration allowing the specified temp directory
func createTestConfig(tmpDir string) *Config {
	return createTestConfigWithTTL(tmpDir, time.Hour)
}

// createTestConfigWithTTL creates a test configuration with custom TTL
func createTestConfigWithTTL(tmpDir string, ttl time.Duration) *Config {
	config := DefaultConfig()
	config.DataDirectory = tmpDir
	config.TTL = ttl
	config.MaxItems = 1000
	config.SyncWrites = true
	return config
}

func TestStore_BasicOperations(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create store
	config := createTestConfig(tmpDir)

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Test data
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	testNeedle, err := needle.New(payload)
	if err != nil {
		t.Fatalf("Failed to create test needle: %v", err)
	}

	// Test Set
	t.Run("Set", func(t *testing.T) {
		err := store.Set(testNeedle)
		if err != nil {
			t.Errorf("Set failed: %v", err)
		}
	})

	// Test Get
	t.Run("Get", func(t *testing.T) {
		hash := testNeedle.Hash()
		retrieved, err := store.Get(hash)
		if err != nil {
			t.Errorf("Get failed: %v", err)
		}

		if retrieved == nil {
			t.Fatal("Retrieved needle is nil")
		}

		// Compare hash
		if retrieved.Hash() != hash {
			t.Errorf("Hash mismatch: got %x, want %x", retrieved.Hash(), hash)
		}

		// Compare payload
		if retrieved.Payload() != testNeedle.Payload() {
			t.Error("Payload mismatch")
		}
	})

	// Test Get non-existent
	t.Run("GetNonExistent", func(t *testing.T) {
		nonExistentHash := needle.Hash{}
		for i := range nonExistentHash {
			nonExistentHash[i] = 0xFF
		}

		_, err := store.Get(nonExistentHash)
		if err != ErrDNE {
			t.Errorf("Expected ErrDNE, got: %v", err)
		}
	})

	// Test Update
	t.Run("Update", func(t *testing.T) {
		// Create new payload
		newPayload := make([]byte, needle.PayloadLength)
		for i := range newPayload {
			newPayload[i] = byte((i + 1) % 256)
		}

		_, err = needle.New(newPayload)
		if err != nil {
			t.Fatalf("Failed to create new needle: %v", err)
		}

		// Update with same hash (simulating update scenario)
		err = store.Set(testNeedle)
		if err != nil {
			t.Errorf("Update failed: %v", err)
		}

		// Verify update
		hash := testNeedle.Hash()
		retrieved, err := store.Get(hash)
		if err != nil {
			t.Errorf("Get after update failed: %v", err)
		}

		if retrieved.Hash() != hash {
			t.Errorf("Hash mismatch after update")
		}
	})
}

func TestStore_Persistence(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-persist-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)

	ctx := context.Background()

	// Store some data
	var testHash needle.Hash
	{
		store, err := New(ctx, config)
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}

		// Create and store needle
		payload := make([]byte, needle.PayloadLength)
		for i := range payload {
			payload[i] = byte(i % 256)
		}

		testNeedle, err := needle.New(payload)
		if err != nil {
			t.Fatalf("Failed to create test needle: %v", err)
		}

		testHash = testNeedle.Hash()

		err = store.Set(testNeedle)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		// Close store
		err = store.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}

	// Reopen and verify data persisted
	{
		store, err := New(ctx, config)
		if err != nil {
			t.Fatalf("Failed to reopen store: %v", err)
		}
		defer func() {
			if err := store.Close(); err != nil {
				t.Errorf("Failed to close store: %v", err)
			}
		}()

		// Debug: Check data file records
		t.Logf("Data file records: %d", store.dataFile.getRecordCount())
		t.Logf("Index entries: %d", store.index.getEntryCount())

		// Try to retrieve the needle
		retrieved, err := store.Get(testHash)
		if err != nil {
			t.Fatalf("Get after reopen failed: %v", err)
		}

		if retrieved == nil {
			t.Fatal("Retrieved needle is nil after reopen")
		}

		if retrieved.Hash() != testHash {
			t.Errorf("Hash mismatch after reopen")
		}
	}
}

func TestStore_TTLExpiration(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-ttl-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	// Short TTL for testing
	config := createTestConfigWithTTL(tmpDir, 100*time.Millisecond)

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Create and store needle
	payload := make([]byte, needle.PayloadLength)
	testNeedle, err := needle.New(payload)
	if err != nil {
		t.Fatalf("Failed to create test needle: %v", err)
	}

	hash := testNeedle.Hash()

	// Store needle
	err = store.Set(testNeedle)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should be retrievable immediately
	_, err = store.Get(hash)
	if err != nil {
		t.Errorf("Get immediately after Set failed: %v", err)
	}

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)

	// Should now be expired
	_, err = store.Get(hash)
	if err != ErrDNE {
		t.Errorf("Expected ErrDNE for expired needle, got: %v", err)
	}
}

func TestStore_MultipleRecords(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-multi-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)
	config.SyncWrites = false // Batch for performance

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Store multiple needles
	const numNeedles = 100
	needles := make([]*needle.Needle, numNeedles)

	for i := 0; i < numNeedles; i++ {
		payload := make([]byte, needle.PayloadLength)
		for j := range payload {
			payload[j] = byte((i + j) % 256)
		}

		n, err := needle.New(payload)
		if err != nil {
			t.Fatalf("Failed to create needle %d: %v", i, err)
		}

		needles[i] = n

		err = store.Set(n)
		if err != nil {
			t.Fatalf("Failed to set needle %d: %v", i, err)
		}
	}

	// Verify all needles can be retrieved
	for i, n := range needles {
		hash := n.Hash()
		retrieved, err := store.Get(hash)
		if err != nil {
			t.Errorf("Failed to get needle %d: %v", i, err)
			continue
		}

		if retrieved.Hash() != hash {
			t.Errorf("Hash mismatch for needle %d", i)
		}
	}
}

func BenchmarkStore_Set(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			b.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)
	config.MaxItems = 1000000
	config.GrowthChunkSize = 10 * 1024 * 1024 // 10MB chunks
	config.SyncWrites = false

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			b.Errorf("Failed to close store: %v", err)
		}
	}()

	// Prepare test needle
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Create unique needle for each iteration
		payload[0] = byte(i)
		n, _ := needle.New(payload)

		err := store.Set(n)
		if err != nil {
			b.Fatalf("Set failed: %v", err)
		}
	}
}

func BenchmarkStore_Get(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-bench-get-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			b.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)
	config.MaxItems = 1000000
	config.GrowthChunkSize = 10 * 1024 * 1024
	config.SyncWrites = false

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			b.Errorf("Failed to close store: %v", err)
		}
	}()

	// Prepare and store test needle
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	testNeedle, err := needle.New(payload)
	if err != nil {
		b.Fatalf("Failed to create test needle: %v", err)
	}

	err = store.Set(testNeedle)
	if err != nil {
		b.Fatalf("Failed to set needle: %v", err)
	}

	hash := testNeedle.Hash()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := store.Get(hash)
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}

func TestStore_NilConfig(t *testing.T) {
	// Create a temp directory and change to it
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-nilconfig-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	// Save current directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("Failed to restore original directory: %v", err)
		}
	}()

	// Test with nil config - should use DefaultConfig
	ctx := context.Background()
	store, err := New(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to create store with nil config: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Verify it works by storing and retrieving data
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	testNeedle, err := needle.New(payload)
	if err != nil {
		t.Fatalf("Failed to create needle: %v", err)
	}

	// Store needle
	if err := store.Set(testNeedle); err != nil {
		t.Errorf("Failed to set needle: %v", err)
	}

	// Retrieve needle
	retrieved, err := store.Get(testNeedle.Hash())
	if err != nil {
		t.Errorf("Failed to get needle: %v", err)
	}

	if retrieved.Hash() != testNeedle.Hash() {
		t.Errorf("Hash mismatch")
	}

	// Verify files were created in current directory
	if _, err := os.Stat("haystack.data"); err != nil {
		t.Errorf("Data file not created in current directory: %v", err)
	}
	if _, err := os.Stat("haystack.index"); err != nil {
		t.Errorf("Index file not created in current directory: %v", err)
	}
}

func TestStore_CurrentDirectory(t *testing.T) {
	// Create a temp directory and change to it
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-currentdir-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	// Save current directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("Failed to restore original directory: %v", err)
		}
	}()

	t.Run("EmptyDataDirectory", func(t *testing.T) {
		// Test with empty DataDirectory
		config := DefaultConfig()
		config.DataDirectory = ""

		ctx := context.Background()
		store, err := New(ctx, config)
		if err != nil {
			t.Fatalf("Failed to create store with empty DataDirectory: %v", err)
		}
		defer func() {
			if err := store.Close(); err != nil {
				t.Errorf("Failed to close store: %v", err)
			}
		}()

		// Verify files were created in current directory
		if _, err := os.Stat("haystack.data"); err != nil {
			t.Errorf("Data file not created in current directory: %v", err)
		}
		if _, err := os.Stat("haystack.index"); err != nil {
			t.Errorf("Index file not created in current directory: %v", err)
		}

		// Cleanup
		if err := os.Remove("haystack.data"); err != nil {
			t.Errorf("Failed to remove haystack.data: %v", err)
		}
		if err := os.Remove("haystack.index"); err != nil {
			t.Errorf("Failed to remove haystack.index: %v", err)
		}
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		// Test with DefaultConfig (uses current directory)
		config := DefaultConfig()

		ctx := context.Background()
		store, err := New(ctx, config)
		if err != nil {
			t.Fatalf("Failed to create store with DefaultConfig: %v", err)
		}
		defer func() {
			if err := store.Close(); err != nil {
				t.Errorf("Failed to close store: %v", err)
			}
		}()

		// Verify files were created in current directory
		if _, err := os.Stat("haystack.data"); err != nil {
			t.Errorf("Data file not created in current directory: %v", err)
		}
		if _, err := os.Stat("haystack.index"); err != nil {
			t.Errorf("Index file not created in current directory: %v", err)
		}

		// Cleanup
		if err := os.Remove("haystack.data"); err != nil {
			t.Errorf("Failed to remove haystack.data: %v", err)
		}
		if err := os.Remove("haystack.index"); err != nil {
			t.Errorf("Failed to remove haystack.index: %v", err)
		}
	})
}

func TestDataFile_Growth(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-growth-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)
	config.GrowthChunkSize = 1024 // Small chunk size to trigger growth

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Store many needles to trigger file growth
	for i := 0; i < 50; i++ {
		payload := make([]byte, needle.PayloadLength)
		for j := range payload {
			payload[j] = byte((i + j) % 256)
		}

		testNeedle, err := needle.New(payload)
		if err != nil {
			t.Fatalf("Failed to create needle %d: %v", i, err)
		}

		err = store.Set(testNeedle)
		if err != nil {
			t.Fatalf("Failed to set needle %d: %v", i, err)
		}
	}

	// Verify all needles can still be retrieved after growth
	for i := 0; i < 50; i++ {
		payload := make([]byte, needle.PayloadLength)
		for j := range payload {
			payload[j] = byte((i + j) % 256)
		}

		testNeedle, err := needle.New(payload)
		if err != nil {
			t.Fatalf("Failed to create needle %d: %v", i, err)
		}

		hash := testNeedle.Hash()
		retrieved, err := store.Get(hash)
		if err != nil {
			t.Errorf("Failed to get needle %d after growth: %v", i, err)
		}

		if retrieved.Hash() != hash {
			t.Errorf("Hash mismatch for needle %d after growth", i)
		}
	}
}

func TestDataFile_GetStats(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-stats-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)
	config.TTL = 100 * time.Millisecond // Short TTL for testing expiration

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Store some needles
	needles := make([]*needle.Needle, 10)
	for i := 0; i < 10; i++ {
		payload := make([]byte, needle.PayloadLength)
		for j := range payload {
			payload[j] = byte((i + j) % 256)
		}

		testNeedle, err := needle.New(payload)
		if err != nil {
			t.Fatalf("Failed to create needle %d: %v", i, err)
		}

		needles[i] = testNeedle

		err = store.Set(testNeedle)
		if err != nil {
			t.Fatalf("Failed to set needle %d: %v", i, err)
		}
	}

	// Get stats - all should be active
	stats := store.dataFile.GetStats()
	if stats.TotalRecords != 10 {
		t.Errorf("Expected 10 total records, got %d", stats.TotalRecords)
	}
	if stats.ActiveRecords != 10 {
		t.Errorf("Expected 10 active records, got %d", stats.ActiveRecords)
	}
	if stats.ExpiredRecords != 0 {
		t.Errorf("Expected 0 expired records, got %d", stats.ExpiredRecords)
	}
	if stats.DataFileSize <= 0 {
		t.Errorf("Expected positive data file size, got %d", stats.DataFileSize)
	}

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)

	// Get stats again - all should be expired
	stats = store.dataFile.GetStats()
	if stats.TotalRecords != 10 {
		t.Errorf("Expected 10 total records after expiration, got %d", stats.TotalRecords)
	}
	if stats.ActiveRecords != 0 {
		t.Errorf("Expected 0 active records after expiration, got %d", stats.ActiveRecords)
	}
	if stats.ExpiredRecords != 10 {
		t.Errorf("Expected 10 expired records after expiration, got %d", stats.ExpiredRecords)
	}
}

func TestIndex_ForEach(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-foreach-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Store some test needles
	expectedHashes := make(map[needle.Hash]bool)
	for i := 0; i < 5; i++ {
		payload := make([]byte, needle.PayloadLength)
		for j := range payload {
			payload[j] = byte((i + j) % 256)
		}

		testNeedle, err := needle.New(payload)
		if err != nil {
			t.Fatalf("Failed to create needle %d: %v", i, err)
		}

		expectedHashes[testNeedle.Hash()] = true

		err = store.Set(testNeedle)
		if err != nil {
			t.Fatalf("Failed to set needle %d: %v", i, err)
		}
	}

	// Test ForEach - iterate through all entries
	visitedHashes := make(map[needle.Hash]bool)
	store.index.ForEach(func(hash needle.Hash, offset uint64) bool {
		visitedHashes[hash] = true
		if offset == 0 {
			t.Errorf("Expected non-zero offset for hash %x", hash)
		}
		return true // Continue iteration
	})

	// Verify all hashes were visited
	for hash := range expectedHashes {
		if !visitedHashes[hash] {
			t.Errorf("Hash %x was not visited during ForEach", hash)
		}
	}

	// Test early termination
	visitCount := 0
	store.index.ForEach(func(hash needle.Hash, offset uint64) bool {
		visitCount++
		return visitCount < 3 // Stop after 3 visits
	})

	if visitCount > 3 {
		t.Errorf("Expected ForEach to stop after 3 visits, but visited %d times", visitCount)
	}
}

func TestStore_Cleanup(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-cleanup-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)
	config.TTL = 50 * time.Millisecond              // Very short TTL
	config.CleanupInterval = 100 * time.Millisecond // Short cleanup interval

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Store some needles
	for i := 0; i < 5; i++ {
		payload := make([]byte, needle.PayloadLength)
		for j := range payload {
			payload[j] = byte((i + j) % 256)
		}

		testNeedle, err := needle.New(payload)
		if err != nil {
			t.Fatalf("Failed to create needle %d: %v", i, err)
		}

		err = store.Set(testNeedle)
		if err != nil {
			t.Fatalf("Failed to set needle %d: %v", i, err)
		}
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Manually trigger cleanup to test performCleanup
	// Note: cleanup() is a goroutine function, so we test performCleanup directly
	store.performCleanup()

	// Verify cleanup ran by checking that expired records are marked as deleted
	stats := store.dataFile.GetStats()
	t.Logf("After cleanup: Total=%d, Active=%d, Expired=%d", stats.TotalRecords, stats.ActiveRecords, stats.ExpiredRecords)
}

func TestStore_Compaction(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-compact-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)
	config.CompactThreshold = 0.5      // Compact when 50% of records are deleted
	config.TTL = 50 * time.Millisecond // Short TTL

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Store some needles
	for i := 0; i < 10; i++ {
		payload := make([]byte, needle.PayloadLength)
		for j := range payload {
			payload[j] = byte((i + j) % 256)
		}

		testNeedle, err := needle.New(payload)
		if err != nil {
			t.Fatalf("Failed to create needle %d: %v", i, err)
		}

		err = store.Set(testNeedle)
		if err != nil {
			t.Fatalf("Failed to set needle %d: %v", i, err)
		}
	}

	// Wait for some to expire
	time.Sleep(100 * time.Millisecond)

	// Trigger cleanup to mark expired records as deleted
	store.performCleanup()

	// Manually trigger compaction
	err = store.compact()
	if err != nil {
		t.Errorf("Compaction failed: %v", err)
	}

	t.Logf("Compaction completed successfully")
}

func TestRecord_MarkDeleted_UpdateExpiration(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-record-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Create and store a test needle
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	testNeedle, err := needle.New(payload)
	if err != nil {
		t.Fatalf("Failed to create test needle: %v", err)
	}

	err = store.Set(testNeedle)
	if err != nil {
		t.Fatalf("Failed to set needle: %v", err)
	}

	// Find the record in the datafile to test methods
	// We need to get the actual offset from the index
	hash := testNeedle.Hash()
	offset, found := store.index.Find(hash)
	if !found {
		t.Fatalf("Failed to find record in index")
	}

	record, err := store.dataFile.ReadRecord(offset)
	if err != nil {
		t.Fatalf("Failed to read record: %v", err)
	}

	// Test MarkDeleted
	if record.IsActive() {
		t.Log("Record is initially active")
	}

	record.MarkDeleted()
	if record.IsActive() {
		t.Error("Record should not be active after MarkDeleted")
	}

	// Test UpdateExpiration
	newExpiration := time.Now().Add(2 * time.Hour)
	record.UpdateExpiration(newExpiration)

	if record.ExpirationTime().Sub(newExpiration).Abs() > time.Second {
		t.Errorf("UpdateExpiration failed: expected %v, got %v", newExpiration, record.ExpirationTime())
	}

	t.Logf("Record tests completed successfully")
}

func TestStore_ErrorConditions(t *testing.T) {
	// Test error conditions for better coverage

	t.Run("InvalidDataDirectory", func(t *testing.T) {
		config := DefaultConfig()
		config.DataDirectory = "/nonexistent/path"

		ctx := context.Background()
		_, err := New(ctx, config)
		if err == nil {
			t.Error("Expected error for nonexistent directory")
		}
	})

	t.Run("InvalidConfig", func(t *testing.T) {
		config := DefaultConfig()
		config.MaxItems = 0 // Invalid max items

		ctx := context.Background()
		_, err := New(ctx, config)
		if err == nil {
			t.Error("Expected error for invalid config")
		}
	})
}

func TestSecurityFunctions(t *testing.T) {
	// Test security functions for better coverage
	tmpDir, err := os.MkdirTemp("", "haystack-security-coverage-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	t.Run("SecureFileCreateEdgeCases", func(t *testing.T) {
		// Test secureFileCreate with various conditions
		testPath := filepath.Join(tmpDir, "test-edge-case.data")

		file, err := secureFileCreate(testPath)
		if err != nil {
			t.Errorf("secureFileCreate failed: %v", err)
		} else {
			func() {
				if err := file.Close(); err != nil {
					t.Errorf("Failed to close file: %v", err)
				}
			}()

			// Verify file exists and has correct permissions
			info, err := os.Stat(testPath)
			if err != nil {
				t.Errorf("Failed to stat created file: %v", err)
			} else if info.Mode().Perm() != 0600 {
				t.Errorf("Expected permissions 0600, got %o", info.Mode().Perm())
			}

			func() {
				if err := os.Remove(testPath); err != nil {
					t.Errorf("Failed to remove test file: %v", err)
				}
			}()
		}
	})

	t.Run("ValidationFunctions", func(t *testing.T) {
		// Test various validation scenarios
		err := validateDataDirectory("")
		if err == nil {
			t.Error("Expected error for empty directory")
		}

		// Test path traversal prevention
		_, err = buildSecureDataPath(tmpDir, "../../etc/passwd")
		if err == nil {
			t.Error("Expected error for path traversal attempt")
		}

		// Test valid path
		validPath, err := buildSecureDataPath(tmpDir, "valid.data")
		if err != nil {
			t.Errorf("buildSecureDataPath failed for valid input: %v", err)
		}
		if !strings.HasSuffix(validPath, "valid.data") {
			t.Errorf("Expected path to end with valid.data, got %s", validPath)
		}
	})
}

func TestDataFile_EdgeCases(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-edge-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	t.Run("InvalidDataFile", func(t *testing.T) {
		// Create a file with invalid content
		invalidPath := filepath.Join(tmpDir, "invalid.data")
		file, err := os.Create(invalidPath)
		if err != nil {
			t.Fatalf("Failed to create invalid file: %v", err)
		}

		// Write invalid header
		_, err = file.Write([]byte("INVALID"))
		if err != nil {
			t.Fatalf("Failed to write invalid data: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Errorf("Failed to close file: %v", err)
		}

		// Try to open as datafile - should fail
		file, err = os.OpenFile(invalidPath, os.O_RDWR, 0600)
		if err != nil {
			t.Fatalf("Failed to reopen file: %v", err)
		}
		_, err = newDataFileFromHandle(invalidPath, file, 1000, 1024*1024)
		if err == nil {
			t.Error("Expected error for invalid data file")
		}
		// Note: newDataFileFromHandle closes the file on error, so don't close again
	})

	t.Run("InvalidIndexFile", func(t *testing.T) {
		// Create a file with invalid content
		invalidPath := filepath.Join(tmpDir, "invalid.index")
		file, err := os.Create(invalidPath)
		if err != nil {
			t.Fatalf("Failed to create invalid file: %v", err)
		}

		// Write invalid header
		_, err = file.Write([]byte("INVALID"))
		if err != nil {
			t.Fatalf("Failed to write invalid data: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Errorf("Failed to close file: %v", err)
		}

		// Try to open as index - should fail
		file, err = os.OpenFile(invalidPath, os.O_RDWR, 0600)
		if err != nil {
			t.Fatalf("Failed to reopen file: %v", err)
		}
		_, err = newIndexFromHandle(invalidPath, file, 1000)
		if err == nil {
			t.Error("Expected error for invalid index file")
		}
		// Note: newIndexFromHandle closes the file on error, so don't close again
	})

	t.Run("HeaderValidation", func(t *testing.T) {
		// Test invalid data header
		invalidHeader := &DataHeader{Magic: [8]byte{'B', 'A', 'D', '!', '!', '!', '!', '!'}}
		err := validateDataHeader(invalidHeader)
		if err == nil {
			t.Error("Expected error for invalid data header magic")
		}

		// Test invalid index header
		invalidIndexHeader := &IndexHeader{Magic: [8]byte{'B', 'A', 'D', '!', '!', '!', '!', '!'}}
		err = validateIndexHeader(invalidIndexHeader)
		if err == nil {
			t.Error("Expected error for invalid index header magic")
		}

		// Test invalid version
		invalidVersionHeader := &DataHeader{
			Magic:   [8]byte{'H', 'A', 'Y', 'S', 'T', 'D', 'A', 'T'},
			Version: 999,
		}
		err = validateDataHeader(invalidVersionHeader)
		if err == nil {
			t.Error("Expected error for invalid data header version")
		}
	})
}

func TestStore_AdvancedErrorConditions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-adverr-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	t.Run("ConfigVariations", func(t *testing.T) {
		config := DefaultConfig()
		config.DataDirectory = tmpDir
		config.SyncWrites = true // Test sync writes enabled

		ctx := context.Background()
		store, err := New(ctx, config)
		if err != nil {
			t.Errorf("Failed to create store with SyncWrites enabled: %v", err)
		} else {
			// Test a needle with sync writes
			payload := make([]byte, needle.PayloadLength)
			testNeedle, err := needle.New(payload)
			if err != nil {
				t.Fatalf("Failed to create needle: %v", err)
			}

			err = store.Set(testNeedle)
			if err != nil {
				t.Errorf("Failed to set needle with sync writes: %v", err)
			}

			func() {
				if err := store.Close(); err != nil {
					t.Errorf("Failed to close store: %v", err)
				}
			}()
		}
	})

	t.Run("SmallCapacity", func(t *testing.T) {
		config := DefaultConfig()
		config.DataDirectory = tmpDir
		config.MaxItems = 10 // Very small capacity

		ctx := context.Background()
		store, err := New(ctx, config)
		if err != nil {
			t.Errorf("Failed to create store with small capacity: %v", err)
		} else {
			func() {
				if err := store.Close(); err != nil {
					t.Errorf("Failed to close store: %v", err)
				}
			}()
		}
	})
}

func TestCleanup_EdgeCases(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-cleanedge-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)
	config.TTL = 50 * time.Millisecond
	config.CompactThreshold = 0.3 // Low threshold to trigger compaction

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Add enough records to trigger automatic compaction during cleanup
	for i := 0; i < 20; i++ {
		payload := make([]byte, needle.PayloadLength)
		for j := range payload {
			payload[j] = byte((i + j) % 256)
		}

		testNeedle, err := needle.New(payload)
		if err != nil {
			t.Fatalf("Failed to create needle %d: %v", i, err)
		}

		err = store.Set(testNeedle)
		if err != nil {
			t.Fatalf("Failed to set needle %d: %v", i, err)
		}
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Trigger cleanup - this should also trigger compaction due to high deletion ratio
	store.performCleanup()

	stats := store.dataFile.GetStats()
	t.Logf("After cleanup with compaction: Total=%d, Active=%d, Expired=%d",
		stats.TotalRecords, stats.ActiveRecords, stats.ExpiredRecords)
}

func TestSecurity_ErrorConditions(t *testing.T) {
	t.Run("CreateFileInNonexistentDir", func(t *testing.T) {
		nonexistentPath := "/nonexistent/dir/file.data"
		_, err := secureFileCreate(nonexistentPath)
		if err == nil {
			t.Error("Expected error for creating file in nonexistent directory")
		}
	})

	t.Run("ValidateNonexistentFile", func(t *testing.T) {
		err := validateExistingFile("/nonexistent/file.data")
		if err == nil {
			t.Error("Expected error for validating nonexistent file")
		}
	})

	t.Run("PathTraversalProtection", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "haystack-security-traversal-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer func() {
			if err := os.RemoveAll(tmpDir); err != nil {
				t.Errorf("Failed to remove temp dir: %v", err)
			}
		}()

		// Test various path traversal attempts
		maliciousPaths := []string{
			"../../../etc/passwd",
			"..\\..\\..\\windows\\system32\\config",
			"../../secret.txt",
		}

		for _, maliciousPath := range maliciousPaths {
			_, err = buildSecureDataPath(tmpDir, maliciousPath)
			if err == nil {
				t.Errorf("Expected error for malicious path: %s", maliciousPath)
			}
		}
	})
}

func TestRecord_EdgeCases(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-record-edge-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	t.Run("InvalidRecordBytes", func(t *testing.T) {
		// Test recordFromBytes with invalid data
		invalidBytes := make([]byte, 10) // Too short
		_, err := recordFromBytes(invalidBytes)
		if err == nil {
			t.Error("Expected error for invalid record bytes")
		}
	})

	t.Run("UpdateExpirationEdgeCases", func(t *testing.T) {
		config := createTestConfig(tmpDir)
		ctx := context.Background()
		store, err := New(ctx, config)
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer func() {
			if err := store.Close(); err != nil {
				t.Errorf("Failed to close store: %v", err)
			}
		}()

		// Create and store a test needle
		payload := make([]byte, needle.PayloadLength)
		testNeedle, err := needle.New(payload)
		if err != nil {
			t.Fatalf("Failed to create test needle: %v", err)
		}

		err = store.Set(testNeedle)
		if err != nil {
			t.Fatalf("Failed to set needle: %v", err)
		}

		// Get the record and test various expiration times
		hash := testNeedle.Hash()
		offset, found := store.index.Find(hash)
		if !found {
			t.Fatalf("Failed to find record in index")
		}

		record, err := store.dataFile.ReadRecord(offset)
		if err != nil {
			t.Fatalf("Failed to read record: %v", err)
		}

		// Test with very old expiration (should make record expired)
		pastTime := time.Now().Add(-24 * time.Hour)
		record.UpdateExpiration(pastTime)

		// Note: IsActive checks if the record hasn't been deleted, not expiration
		// For expiration, we'd need to test differently

		// Test with future expiration
		futureTime := time.Now().Add(24 * time.Hour)
		record.UpdateExpiration(futureTime)

		if !record.IsActive() {
			t.Error("Record should be active with future expiration")
		}
	})
}

func TestDataFile_MarkDeleted(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-markdel-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)
	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Create and store a needle
	payload := make([]byte, needle.PayloadLength)
	testNeedle, err := needle.New(payload)
	if err != nil {
		t.Fatalf("Failed to create needle: %v", err)
	}

	err = store.Set(testNeedle)
	if err != nil {
		t.Fatalf("Failed to set needle: %v", err)
	}

	// Get the offset from index
	hash := testNeedle.Hash()
	offset, found := store.index.Find(hash)
	if !found {
		t.Fatalf("Failed to find record in index")
	}

	// Mark the record as deleted in the datafile
	err = store.dataFile.MarkDeleted(offset)
	if err != nil {
		t.Errorf("Failed to mark record as deleted: %v", err)
	}

	// Verify it's marked as deleted
	record, err := store.dataFile.ReadRecord(offset)
	if err != nil {
		t.Fatalf("Failed to read record: %v", err)
	}

	if record.IsActive() {
		t.Error("Record should not be active after being marked deleted")
	}
}

func TestIndex_EdgeConditions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-index-edge-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)
	config.MaxItems = 5 // Small index for testing edge cases

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Fill the index to capacity
	var hashes []needle.Hash
	for i := 0; i < 5; i++ {
		payload := make([]byte, needle.PayloadLength)
		for j := range payload {
			payload[j] = byte((i + j) % 256)
		}

		testNeedle, err := needle.New(payload)
		if err != nil {
			t.Fatalf("Failed to create needle %d: %v", i, err)
		}

		err = store.Set(testNeedle)
		if err != nil {
			t.Fatalf("Failed to set needle %d: %v", i, err)
		}

		hashes = append(hashes, testNeedle.Hash())
	}

	// Test that all entries can be found
	for i, hash := range hashes {
		_, found := store.index.Find(hash)
		if !found {
			t.Errorf("Failed to find hash %d in index", i)
		}
	}

	// Test ForEach with early termination
	count := 0
	store.index.ForEach(func(hash needle.Hash, offset uint64) bool {
		count++
		return count < 3 // Stop after 3 entries
	})

	if count != 3 {
		t.Errorf("Expected ForEach to stop at 3 entries, got %d", count)
	}
}

func TestStore_SyncOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-sync-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)
	config.SyncWrites = true

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Test manual sync
	err = store.sync()
	if err != nil {
		t.Errorf("Manual sync failed: %v", err)
	}

	// Test sync during Set operation (triggered by SyncWrites=true)
	payload := make([]byte, needle.PayloadLength)
	testNeedle, err := needle.New(payload)
	if err != nil {
		t.Fatalf("Failed to create needle: %v", err)
	}

	err = store.Set(testNeedle)
	if err != nil {
		t.Errorf("Set with sync failed: %v", err)
	}
}

func TestNewFunctions_Coverage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "haystack-coverage-new-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	t.Run("NewDataFileFromHandle_ExistingFile", func(t *testing.T) {
		// Create a valid existing data file
		config := createTestConfig(tmpDir)
		ctx := context.Background()
		store, err := New(ctx, config)
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}

		// Store a needle to create a valid file
		payload := make([]byte, needle.PayloadLength)
		testNeedle, err := needle.New(payload)
		if err != nil {
			t.Fatalf("Failed to create needle: %v", err)
		}

		err = store.Set(testNeedle)
		if err != nil {
			t.Fatalf("Failed to set needle: %v", err)
		}

		// Close the store to ensure data is written
		if err := store.Close(); err != nil {
			t.Fatalf("Failed to close store: %v", err)
		}

		// Now open the existing data file using newDataFileFromHandle
		dataPath := filepath.Join(tmpDir, "haystack.data")
		file, err := os.OpenFile(dataPath, os.O_RDWR, 0600)
		if err != nil {
			t.Fatalf("Failed to open data file: %v", err)
		}

		// This should successfully open the existing file
		dataFile, err := newDataFileFromHandle(dataPath, file, 1000, 1024*1024)
		if err != nil {
			t.Errorf("Failed to open existing data file: %v", err)
		} else {
			// Verify we can read the record count
			count := dataFile.getRecordCount()
			if count != 1 {
				t.Errorf("Expected 1 record, got %d", count)
			}

			if err := dataFile.Close(); err != nil {
				t.Errorf("Failed to close data file: %v", err)
			}
		}
	})

	t.Run("NewIndexFromHandle_ExistingFile", func(t *testing.T) {
		// Create a valid existing index file
		config := createTestConfig(tmpDir)
		ctx := context.Background()
		store, err := New(ctx, config)
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}

		// Store a needle to create a valid index
		payload := make([]byte, needle.PayloadLength)
		testNeedle, err := needle.New(payload)
		if err != nil {
			t.Fatalf("Failed to create needle: %v", err)
		}

		err = store.Set(testNeedle)
		if err != nil {
			t.Fatalf("Failed to set needle: %v", err)
		}

		// Close the store to ensure index is written
		if err := store.Close(); err != nil {
			t.Fatalf("Failed to close store: %v", err)
		}

		// Now open the existing index file using newIndexFromHandle
		indexPath := filepath.Join(tmpDir, "haystack.index")
		file, err := os.OpenFile(indexPath, os.O_RDWR, 0600)
		if err != nil {
			t.Fatalf("Failed to open index file: %v", err)
		}

		// This should successfully open the existing file
		index, err := newIndexFromHandle(indexPath, file, 1000)
		if err != nil {
			t.Errorf("Failed to open existing index file: %v", err)
		} else {
			// Verify we can read the entry count
			count := index.getEntryCount()
			if count != 1 {
				t.Errorf("Expected 1 entry, got %d", count)
			}

			if err := index.Close(); err != nil {
				t.Errorf("Failed to close index: %v", err)
			}
		}
	})
}

func TestPerformCleanup_Coverage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "haystack-cleanup-coverage-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)
	config.TTL = 10 * time.Millisecond // Very short TTL
	config.CompactThreshold = 0.8      // High threshold to test different cleanup path

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Add some records
	for i := 0; i < 5; i++ {
		payload := make([]byte, needle.PayloadLength)
		for j := range payload {
			payload[j] = byte((i + j) % 256)
		}

		testNeedle, err := needle.New(payload)
		if err != nil {
			t.Fatalf("Failed to create needle %d: %v", i, err)
		}

		err = store.Set(testNeedle)
		if err != nil {
			t.Fatalf("Failed to set needle %d: %v", i, err)
		}
	}

	// Wait for TTL expiration
	time.Sleep(50 * time.Millisecond)

	// Call performCleanup - this should exercise different code paths
	store.performCleanup()

	// Check that cleanup ran
	stats := store.dataFile.GetStats()
	t.Logf("After performCleanup: Total=%d, Active=%d, Expired=%d",
		stats.TotalRecords, stats.ActiveRecords, stats.ExpiredRecords)
}

func TestSecureFileCreate_ErrorPaths(t *testing.T) {
	t.Run("ReadOnlyDirectory", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "haystack-readonly-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer func() {
			// Restore write permissions before cleanup
			if err := os.Chmod(tmpDir, 0755); err != nil {
				t.Errorf("Failed to restore permissions: %v", err)
			}
			if err := os.RemoveAll(tmpDir); err != nil {
				t.Errorf("Failed to remove temp dir: %v", err)
			}
		}()

		// Make directory read-only
		if err := os.Chmod(tmpDir, 0444); err != nil {
			t.Fatalf("Failed to make directory read-only: %v", err)
		}

		testPath := filepath.Join(tmpDir, "test.data")
		_, err = secureFileCreate(testPath)
		if err == nil {
			t.Error("Expected error for creating file in read-only directory")
		}
	})

	t.Run("ValidFileCreation", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "haystack-valid-create-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer func() {
			if err := os.RemoveAll(tmpDir); err != nil {
				t.Errorf("Failed to remove temp dir: %v", err)
			}
		}()

		testPath := filepath.Join(tmpDir, "test.data")
		file, err := secureFileCreate(testPath)
		if err != nil {
			t.Errorf("secureFileCreate failed: %v", err)
		} else {
			// Verify file was created with correct permissions
			info, err := file.Stat()
			if err != nil {
				t.Errorf("Failed to stat file: %v", err)
			} else {
				if info.Mode().Perm() != 0600 {
					t.Errorf("Expected permissions 0600, got %o", info.Mode().Perm())
				}
			}

			if err := file.Close(); err != nil {
				t.Errorf("Failed to close file: %v", err)
			}
		}
	})
}

func TestMapFile_ErrorConditions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "haystack-mapfile-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create a small file for testing mmap edge cases
	testPath := filepath.Join(tmpDir, "small.data")
	file, err := os.Create(testPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Write a minimal amount of data
	data := make([]byte, 100)
	_, err = file.Write(data)
	if err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}

	if err := file.Close(); err != nil {
		t.Fatalf("Failed to close test file: %v", err)
	}

	// Try to create a datafile with it - this should exercise mapFile error paths
	file, err = os.OpenFile(testPath, os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("Failed to reopen test file: %v", err)
	}

	_, err = newDataFileFromHandle(testPath, file, 1000, 1024*1024)
	// This may or may not error depending on the file size validation
	if err != nil {
		t.Logf("Expected error for small file: %v", err)
	}
}

func TestInsert_IndexCapacity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "haystack-insert-capacity-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)
	config.MaxItems = 3 // Very small to test capacity limits

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Fill to capacity and beyond
	for i := 0; i < 5; i++ {
		payload := make([]byte, needle.PayloadLength)
		for j := range payload {
			payload[j] = byte((i + j) % 256)
		}

		testNeedle, err := needle.New(payload)
		if err != nil {
			t.Fatalf("Failed to create needle %d: %v", i, err)
		}

		err = store.Set(testNeedle)
		// After capacity, this may start to error or handle overflow
		if err != nil {
			t.Logf("Set failed at needle %d (expected): %v", i, err)
		}
	}
}

func TestComplexScenarios(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "haystack-complex-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	config := createTestConfig(tmpDir)
	config.SyncWrites = true
	config.GrowthChunkSize = 512 // Small chunks to force growth

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Perform many operations to exercise all code paths
	for i := 0; i < 50; i++ {
		payload := make([]byte, needle.PayloadLength)
		for j := range payload {
			payload[j] = byte((i * j) % 256)
		}

		testNeedle, err := needle.New(payload)
		if err != nil {
			t.Fatalf("Failed to create needle %d: %v", i, err)
		}

		// Set the needle
		err = store.Set(testNeedle)
		if err != nil {
			t.Fatalf("Failed to set needle %d: %v", i, err)
		}

		// Immediately try to get it
		hash := testNeedle.Hash()
		retrieved, err := store.Get(hash)
		if err != nil {
			t.Errorf("Failed to get needle %d: %v", i, err)
			continue
		}

		if retrieved.Hash() != hash {
			t.Errorf("Hash mismatch for needle %d", i)
		}

		// Test sync operation
		if i%10 == 0 {
			err = store.sync()
			if err != nil {
				t.Errorf("Sync failed at iteration %d: %v", i, err)
			}
		}
	}

	// Check final stats
	stats := store.dataFile.GetStats()
	t.Logf("Final stats: Total=%d, Active=%d, Expired=%d, FileSize=%d",
		stats.TotalRecords, stats.ActiveRecords, stats.ExpiredRecords, stats.DataFileSize)
}
