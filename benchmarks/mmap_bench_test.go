package benchmarks

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nomasters/haystack/storage/mmap"
)

// BenchmarkMmap_Throughput tests throughput with memory-mapped storage
func BenchmarkMmap_Throughput(b *testing.B) {
	// Create temp directory for mmap files
	tmpDir, err := os.MkdirTemp("", "haystack-mmap-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mmap storage backend
	ctx := context.Background()
	config := &mmap.Config{
		DataPath:         filepath.Join(tmpDir, "bench.data"),
		IndexPath:        filepath.Join(tmpDir, "bench.index"),
		TTL:              time.Hour,
		MaxItems:         10000000, // 10M items
		CompactThreshold: 0.25,
		GrowthChunkSize:  10 * 1024 * 1024, // 10MB chunks
		SyncWrites:       false, // Batch for performance
		CleanupInterval:  time.Hour,
	}
	
	storage, err := mmap.New(ctx, config)
	if err != nil {
		b.Fatalf("Failed to create mmap storage: %v", err)
	}
	defer storage.Close()

	// Run the same throughput tests as memory storage
	serverAddr := startServerWithStorage(b, storage)
	
	// Test different client configurations
	scenarios := []struct {
		name        string
		numClients  int
		poolSize    int
		readRatio   float64
	}{
		{"mmap_single_client_writes", 1, 5, 0.0},
		{"mmap_single_client_reads", 1, 5, 1.0},
		{"mmap_single_client_mixed", 1, 5, 0.7},
		{"mmap_multi_client_writes", 10, 5, 0.0},
		{"mmap_multi_client_mixed", 10, 5, 0.7},
	}
	
	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			runE2EBenchmark(b, serverAddr, scenario.numClients, scenario.poolSize, scenario.readRatio)
		})
	}
}

// BenchmarkMmap_vs_Memory compares mmap and memory storage directly
func BenchmarkMmap_vs_Memory(b *testing.B) {
	// Create temp directory for mmap files
	tmpDir, err := os.MkdirTemp("", "haystack-compare-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	b.Run("memory_storage", func(b *testing.B) {
		serverAddr := startE2EServer(b)
		measureThroughput(b, serverAddr, 10) // 10 clients
	})

	b.Run("mmap_storage", func(b *testing.B) {
		// Create mmap storage
		ctx := context.Background()
		config := &mmap.Config{
			DataPath:         filepath.Join(tmpDir, "compare.data"),
			IndexPath:        filepath.Join(tmpDir, "compare.index"),
			TTL:              time.Hour,
			MaxItems:         10000000,
			CompactThreshold: 0.25,
			GrowthChunkSize:  10 * 1024 * 1024,
			SyncWrites:       false,
			CleanupInterval:  time.Hour,
		}
		
		storage, err := mmap.New(ctx, config)
		if err != nil {
			b.Fatalf("Failed to create mmap storage: %v", err)
		}
		defer storage.Close()

		serverAddr := startServerWithStorage(b, storage)
		measureThroughput(b, serverAddr, 10) // 10 clients
	})
}

// BenchmarkMmap_Persistence tests performance with pre-existing data
func BenchmarkMmap_Persistence(b *testing.B) {
	// Create persistent directory (not temp)
	benchDir := filepath.Join(os.TempDir(), "haystack-persist-bench")
	os.MkdirAll(benchDir, 0755)
	defer os.RemoveAll(benchDir)

	ctx := context.Background()
	config := &mmap.Config{
		DataPath:         filepath.Join(benchDir, "persist.data"),
		IndexPath:        filepath.Join(benchDir, "persist.index"),
		TTL:              24 * time.Hour, // Long TTL
		MaxItems:         10000000,
		CompactThreshold: 0.25,
		GrowthChunkSize:  50 * 1024 * 1024, // 50MB chunks
		SyncWrites:       false,
		CleanupInterval:  time.Hour,
	}

	// Pre-populate with data
	b.Run("populate", func(b *testing.B) {
		storage, err := mmap.New(ctx, config)
		if err != nil {
			b.Fatalf("Failed to create mmap storage: %v", err)
		}
		
		serverAddr := startServerWithStorage(b, storage)
		
		// Write 1M records
		b.ResetTimer()
		runE2EBenchmark(b, serverAddr, 10, 5, 0.0) // Write only
		b.StopTimer()
		
		storage.Close()
	})

	// Test read performance with pre-existing data
	b.Run("read_existing", func(b *testing.B) {
		storage, err := mmap.New(ctx, config)
		if err != nil {
			b.Fatalf("Failed to reopen mmap storage: %v", err)
		}
		defer storage.Close()
		
		serverAddr := startServerWithStorage(b, storage)
		
		// Read from pre-populated data
		b.ResetTimer()
		runE2EBenchmark(b, serverAddr, 10, 5, 1.0) // Read only
	})
}