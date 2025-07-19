package client

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nomasters/haystack/needle"
	"github.com/nomasters/haystack/server"
	"github.com/nomasters/haystack/storage/memory"
)

// BenchmarkClient_Set benchmarks client SET operations
func BenchmarkClient_Set(b *testing.B) {
	serverAddr := startBenchServer(b)
	
	// Create client
	config := DefaultConfig(serverAddr)
	client, err := New(config)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx := context.Background()
	
	// Create test needle
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	testNeedle, err := needle.New(payload)
	if err != nil {
		b.Fatalf("Failed to create test needle: %v", err)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		err := client.Set(ctx, testNeedle)
		if err != nil {
			b.Fatalf("SET operation failed: %v", err)
		}
	}
}

// BenchmarkClient_Get benchmarks client GET operations
func BenchmarkClient_Get(b *testing.B) {
	serverAddr := startBenchServer(b)
	
	// Create client
	config := DefaultConfig(serverAddr)
	client, err := New(config)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx := context.Background()
	
	// Create and store test needle
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	testNeedle, err := needle.New(payload)
	if err != nil {
		b.Fatalf("Failed to create test needle: %v", err)
	}
	
	// Store the needle first
	err = client.Set(ctx, testNeedle)
	if err != nil {
		b.Fatalf("Failed to store test needle: %v", err)
	}
	
	hash := testNeedle.Hash()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := client.Get(ctx, hash)
		if err != nil {
			b.Fatalf("GET operation failed: %v", err)
		}
	}
}

// BenchmarkClient_SetBytes benchmarks raw bytes SET operations
func BenchmarkClient_SetBytes(b *testing.B) {
	serverAddr := startBenchServer(b)
	
	// Create client
	config := DefaultConfig(serverAddr)
	client, err := New(config)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx := context.Background()
	
	// Create test needle bytes
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	testNeedle, err := needle.New(payload)
	if err != nil {
		b.Fatalf("Failed to create test needle: %v", err)
	}
	needleBytes := testNeedle.Bytes()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		err := client.SetBytes(ctx, needleBytes)
		if err != nil {
			b.Fatalf("SetBytes operation failed: %v", err)
		}
	}
}

// BenchmarkClient_GetBytes benchmarks raw bytes GET operations
func BenchmarkClient_GetBytes(b *testing.B) {
	serverAddr := startBenchServer(b)
	
	// Create client
	config := DefaultConfig(serverAddr)
	client, err := New(config)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx := context.Background()
	
	// Create and store test needle
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	testNeedle, err := needle.New(payload)
	if err != nil {
		b.Fatalf("Failed to create test needle: %v", err)
	}
	
	// Store the needle first
	err = client.Set(ctx, testNeedle)
	if err != nil {
		b.Fatalf("Failed to store test needle: %v", err)
	}
	
	hash := testNeedle.Hash()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := client.GetBytes(ctx, hash[:])
		if err != nil {
			b.Fatalf("GetBytes operation failed: %v", err)
		}
	}
}

// BenchmarkClient_Concurrent_Set benchmarks concurrent SET operations
func BenchmarkClient_Concurrent_Set(b *testing.B) {
	serverAddr := startBenchServer(b)
	
	// Create client
	config := DefaultConfig(serverAddr)
	config.MaxConnections = 50 // Allow more connections for concurrency
	client, err := New(config)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	// Create test needle
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	testNeedle, err := needle.New(payload)
	if err != nil {
		b.Fatalf("Failed to create test needle: %v", err)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			err := client.Set(ctx, testNeedle)
			if err != nil {
				b.Errorf("SET operation failed: %v", err)
				return
			}
		}
	})
}

// BenchmarkClient_Concurrent_Get benchmarks concurrent GET operations
func BenchmarkClient_Concurrent_Get(b *testing.B) {
	serverAddr := startBenchServer(b)
	
	// Create client
	config := DefaultConfig(serverAddr)
	config.MaxConnections = 50 // Allow more connections for concurrency
	client, err := New(config)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx := context.Background()
	
	// Create and store test needle
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	testNeedle, err := needle.New(payload)
	if err != nil {
		b.Fatalf("Failed to create test needle: %v", err)
	}
	
	// Store the needle first
	err = client.Set(ctx, testNeedle)
	if err != nil {
		b.Fatalf("Failed to store test needle: %v", err)
	}
	
	hash := testNeedle.Hash()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			_, err := client.Get(ctx, hash)
			if err != nil {
				b.Errorf("GET operation failed: %v", err)
				return
			}
		}
	})
}

// BenchmarkClient_Mixed_Workload benchmarks mixed read/write operations
func BenchmarkClient_Mixed_Workload(b *testing.B) {
	serverAddr := startBenchServer(b)
	
	// Create client
	config := DefaultConfig(serverAddr)
	config.MaxConnections = 50
	client, err := New(config)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx := context.Background()
	
	// Pre-populate with some data
	var storedHashes []needle.Hash
	for i := 0; i < 100; i++ {
		payload := make([]byte, needle.PayloadLength)
		for j := range payload {
			payload[j] = byte((i + j) % 256)
		}
		
		testNeedle, err := needle.New(payload)
		if err != nil {
			b.Fatalf("Failed to create test needle: %v", err)
		}
		
		err = client.Set(ctx, testNeedle)
		if err != nil {
			b.Fatalf("Failed to store test needle: %v", err)
		}
		
		storedHashes = append(storedHashes, testNeedle.Hash())
	}
	
	var opCounter int64
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Run mixed operations (70% reads, 30% writes)
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			opNum := atomic.AddInt64(&opCounter, 1)
			
			if opNum%10 < 7 { // 70% reads
				// GET operation
				hashIndex := int(opNum) % len(storedHashes)
				hash := storedHashes[hashIndex]
				
				_, err := client.Get(ctx, hash)
				if err != nil {
					b.Errorf("GET operation failed: %v", err)
					return
				}
			} else { // 30% writes
				// SET operation
				payload := make([]byte, needle.PayloadLength)
				for i := range payload {
					payload[i] = byte((int(opNum) + i) % 256)
				}
				
				testNeedle, err := needle.New(payload)
				if err != nil {
					b.Errorf("Failed to create test needle: %v", err)
					return
				}
				
				err = client.Set(ctx, testNeedle)
				if err != nil {
					b.Errorf("SET operation failed: %v", err)
					return
				}
			}
		}
	})
}

// BenchmarkClient_ConnectionPool benchmarks connection pool performance
func BenchmarkClient_ConnectionPool(b *testing.B) {
	serverAddr := startBenchServer(b)
	
	// Test different pool sizes
	for _, poolSize := range []int{1, 5, 10, 25, 50} {
		b.Run(fmt.Sprintf("pool_size_%d", poolSize), func(b *testing.B) {
			config := DefaultConfig(serverAddr)
			config.MaxConnections = poolSize
			client, err := New(config)
			if err != nil {
				b.Fatalf("Failed to create client: %v", err)
			}
			defer client.Close()
			
			// Create test needle
			payload := make([]byte, needle.PayloadLength)
			for i := range payload {
				payload[i] = byte(i % 256)
			}
			testNeedle, err := needle.New(payload)
			if err != nil {
				b.Fatalf("Failed to create test needle: %v", err)
			}
			
			b.ResetTimer()
			
			b.RunParallel(func(pb *testing.PB) {
				ctx := context.Background()
				for pb.Next() {
					err := client.Set(ctx, testNeedle)
					if err != nil {
						b.Errorf("SET operation failed: %v", err)
						return
					}
				}
			})
		})
	}
}

// BenchmarkClient_HighThroughput measures sustained throughput
func BenchmarkClient_HighThroughput(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping high throughput benchmark in short mode")
	}
	
	serverAddr := startBenchServer(b)
	
	// Create client optimized for high throughput
	config := DefaultConfig(serverAddr)
	config.MaxConnections = 100
	config.ReadTimeout = 100 * time.Millisecond
	config.WriteTimeout = 100 * time.Millisecond
	client, err := New(config)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	// Create test needle
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	testNeedle, err := needle.New(payload)
	if err != nil {
		b.Fatalf("Failed to create test needle: %v", err)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Measure operations per second
	start := time.Now()
	var ops int64
	
	var wg sync.WaitGroup
	
	// Launch many concurrent goroutines
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			
			for j := 0; j < b.N/50; j++ {
				err := client.Set(ctx, testNeedle)
				if err != nil {
					b.Errorf("SET operation failed: %v", err)
					return
				}
				atomic.AddInt64(&ops, 1)
			}
		}()
	}
	
	wg.Wait()
	
	duration := time.Since(start)
	opsPerSec := float64(ops) / duration.Seconds()
	
	b.ReportMetric(opsPerSec, "ops/sec")
}

// startBenchServer starts a test server for benchmarking and returns its address
func startBenchServer(b *testing.B) string {
	// Create storage backend with larger capacity for benchmarks
	ctx := context.Background()
	storage := memory.New(ctx, time.Hour, 1000000)
	
	// Create server
	srv := server.New(&server.Config{Storage: storage})
	
	// Find available port
	listener, err := net.ListenPacket("udp", ":0")
	if err != nil {
		b.Fatalf("Failed to find available port: %v", err)
	}
	addr := listener.LocalAddr().String()
	listener.Close()
	
	// Start server
	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			b.Logf("Benchmark server stopped: %v", err)
		}
	}()
	
	// Give server time to start
	time.Sleep(10 * time.Millisecond)
	
	// Clean up when benchmark completes
	b.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		storage.Close()
	})
	
	return addr
}