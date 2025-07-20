package mmap

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nomasters/haystack/needle"
)

func TestStore_BasicOperations(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store
	config := &Config{
		DataPath:         filepath.Join(tmpDir, "test.data"),
		IndexPath:        filepath.Join(tmpDir, "test.index"),
		TTL:              time.Hour,
		MaxItems:         1000,
		CompactThreshold: 0.25,
		GrowthChunkSize:  1024 * 1024,
		SyncWrites:       true,
		CleanupInterval:  time.Hour,
	}

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

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
	defer os.RemoveAll(tmpDir)

	config := &Config{
		DataPath:         filepath.Join(tmpDir, "persist.data"),
		IndexPath:        filepath.Join(tmpDir, "persist.index"),
		TTL:              time.Hour,
		MaxItems:         1000,
		CompactThreshold: 0.25,
		GrowthChunkSize:  1024 * 1024,
		SyncWrites:       true,
		CleanupInterval:  time.Hour,
	}

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
		defer store.Close()

		// Debug: Check data file records
		t.Logf("Data file records: %d", store.dataFile.header.RecordCount)
		t.Logf("Index entries: %d", store.index.header.EntryCount)

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
	defer os.RemoveAll(tmpDir)

	// Short TTL for testing
	config := &Config{
		DataPath:         filepath.Join(tmpDir, "ttl.data"),
		IndexPath:        filepath.Join(tmpDir, "ttl.index"),
		TTL:              100 * time.Millisecond, // Very short TTL
		MaxItems:         1000,
		CompactThreshold: 0.25,
		GrowthChunkSize:  1024 * 1024,
		SyncWrites:       true,
		CleanupInterval:  time.Hour,
	}

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

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
	defer os.RemoveAll(tmpDir)

	config := &Config{
		DataPath:         filepath.Join(tmpDir, "multi.data"),
		IndexPath:        filepath.Join(tmpDir, "multi.index"),
		TTL:              time.Hour,
		MaxItems:         1000,
		CompactThreshold: 0.25,
		GrowthChunkSize:  1024 * 1024,
		SyncWrites:       false, // Batch for performance
		CleanupInterval:  time.Hour,
	}

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

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
	defer os.RemoveAll(tmpDir)

	config := &Config{
		DataPath:         filepath.Join(tmpDir, "bench.data"),
		IndexPath:        filepath.Join(tmpDir, "bench.index"),
		TTL:              time.Hour,
		MaxItems:         1000000,
		CompactThreshold: 0.25,
		GrowthChunkSize:  10 * 1024 * 1024, // 10MB chunks
		SyncWrites:       false,
		CleanupInterval:  time.Hour,
	}

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

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
	defer os.RemoveAll(tmpDir)

	config := &Config{
		DataPath:         filepath.Join(tmpDir, "bench.data"),
		IndexPath:        filepath.Join(tmpDir, "bench.index"),
		TTL:              time.Hour,
		MaxItems:         1000000,
		CompactThreshold: 0.25,
		GrowthChunkSize:  10 * 1024 * 1024,
		SyncWrites:       false,
		CleanupInterval:  time.Hour,
	}

	ctx := context.Background()
	store, err := New(ctx, config)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

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