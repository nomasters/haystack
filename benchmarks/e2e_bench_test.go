package benchmarks

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nomasters/haystack/client"
	"github.com/nomasters/haystack/needle"
	"github.com/nomasters/haystack/server"
	"github.com/nomasters/haystack/storage"
	"github.com/nomasters/haystack/storage/memory"
)

// BenchmarkE2E_Realistic_Workload simulates a realistic production workload
func BenchmarkE2E_Realistic_Workload(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping end-to-end benchmark in short mode")
	}
	
	serverAddr := startE2EServer(b)
	
	// Test different client configurations
	scenarios := []struct {
		name        string
		numClients  int
		poolSize    int
		readRatio   float64 // 0.0 = all writes, 1.0 = all reads
	}{
		{"single_client_writes", 1, 5, 0.0},
		{"single_client_reads", 1, 5, 1.0},
		{"single_client_mixed", 1, 5, 0.7},
		{"multi_client_writes", 10, 5, 0.0},
		{"multi_client_reads", 10, 5, 1.0},
		{"multi_client_mixed", 10, 5, 0.7},
		{"high_concurrency", 25, 10, 0.8},
		{"stress_test", 50, 20, 0.6},
	}
	
	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			runE2EBenchmark(b, serverAddr, scenario.numClients, scenario.poolSize, scenario.readRatio)
		})
	}
}

// BenchmarkE2E_Throughput_Scale tests throughput scaling with different client counts
func BenchmarkE2E_Throughput_Scale(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping throughput scale benchmark in short mode")
	}
	
	serverAddr := startE2EServer(b)
	
	// Test scaling from 1 to 100 clients
	clientCounts := []int{1, 5, 10, 25, 50, 100}
	
	for _, numClients := range clientCounts {
		b.Run(fmt.Sprintf("clients_%d", numClients), func(b *testing.B) {
			measureThroughput(b, serverAddr, numClients)
		})
	}
}

// BenchmarkE2E_Latency measures end-to-end latency
func BenchmarkE2E_Latency(b *testing.B) {
	serverAddr := startE2EServer(b)
	
	// Create single client for latency measurement
	config := client.DefaultConfig(serverAddr)
	c, err := client.New(config)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()
	
	ctx := context.Background()
	
	// Create test data
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	testNeedle, err := needle.New(payload)
	if err != nil {
		b.Fatalf("Failed to create test needle: %v", err)
	}
	
	b.Run("SET_latency", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			start := time.Now()
			err := c.Set(ctx, testNeedle)
			latency := time.Since(start)
			
			if err != nil {
				b.Fatalf("SET operation failed: %v", err)
			}
			
			b.ReportMetric(float64(latency.Nanoseconds()), "ns/op")
		}
	})
	
	// Store data for GET latency test
	err = c.Set(ctx, testNeedle)
	if err != nil {
		b.Fatalf("Failed to store test data: %v", err)
	}
	
	hash := testNeedle.Hash()
	
	b.Run("GET_latency", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			start := time.Now()
			_, err := c.Get(ctx, hash)
			latency := time.Since(start)
			
			if err != nil {
				b.Fatalf("GET operation failed: %v", err)
			}
			
			b.ReportMetric(float64(latency.Nanoseconds()), "ns/op")
		}
	})
}

// BenchmarkE2E_Memory_Pressure tests performance under memory pressure
func BenchmarkE2E_Memory_Pressure(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping memory pressure benchmark in short mode")
	}
	
	serverAddr := startE2EServer(b)
	
	// Create client
	config := client.DefaultConfig(serverAddr)
	config.MaxConnections = 20
	c, err := client.New(config)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()
	
	ctx := context.Background()
	
	// Fill memory with data first
	b.Log("Pre-populating server with data...")
	var storedHashes []needle.Hash
	
	for i := 0; i < 10000; i++ {
		payload := make([]byte, needle.PayloadLength)
		for j := range payload {
			payload[j] = byte((i + j) % 256)
		}
		
		testNeedle, err := needle.New(payload)
		if err != nil {
			b.Fatalf("Failed to create test needle: %v", err)
		}
		
		err = c.Set(ctx, testNeedle)
		if err != nil {
			b.Fatalf("Failed to store test needle: %v", err)
		}
		
		storedHashes = append(storedHashes, testNeedle.Hash())
		
		if i%1000 == 0 {
			b.Logf("Stored %d needles", i)
		}
	}
	
	b.Log("Starting benchmark with pre-populated data...")
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Now benchmark with memory pressure
	for i := 0; i < b.N; i++ {
		// Mix of reads and writes
		if i%3 == 0 {
			// Write new data
			payload := make([]byte, needle.PayloadLength)
			for j := range payload {
				payload[j] = byte((i + j) % 256)
			}
			
			testNeedle, err := needle.New(payload)
			if err != nil {
				b.Fatalf("Failed to create test needle: %v", err)
			}
			
			err = c.Set(ctx, testNeedle)
			if err != nil {
				b.Fatalf("SET operation failed: %v", err)
			}
		} else {
			// Read existing data
			hashIndex := i % len(storedHashes)
			hash := storedHashes[hashIndex]
			
			_, err := c.Get(ctx, hash)
			if err != nil {
				b.Fatalf("GET operation failed: %v", err)
			}
		}
	}
}

// runE2EBenchmark runs a configurable end-to-end benchmark
func runE2EBenchmark(b *testing.B, serverAddr string, numClients, poolSize int, readRatio float64) {
	// Create multiple clients
	clients := make([]*client.Client, numClients)
	for i := 0; i < numClients; i++ {
		config := client.DefaultConfig(serverAddr)
		config.MaxConnections = poolSize
		c, err := client.New(config)
		if err != nil {
			b.Fatalf("Failed to create client %d: %v", i, err)
		}
		defer c.Close()
		clients[i] = c
	}
	
	// Pre-populate with some data for reads
	ctx := context.Background()
	var storedHashes []needle.Hash
	
	if readRatio > 0 {
		for i := 0; i < 100; i++ {
			payload := make([]byte, needle.PayloadLength)
			for j := range payload {
				payload[j] = byte((i + j) % 256)
			}
			
			testNeedle, err := needle.New(payload)
			if err != nil {
				b.Fatalf("Failed to create test needle: %v", err)
			}
			
			err = clients[0].Set(ctx, testNeedle)
			if err != nil {
				b.Fatalf("Failed to store test needle: %v", err)
			}
			
			storedHashes = append(storedHashes, testNeedle.Hash())
		}
	}
	
	var opCounter int64
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Run benchmark
	var wg sync.WaitGroup
	opsPerClient := b.N / numClients
	if opsPerClient == 0 {
		opsPerClient = 1
	}
	
	start := time.Now()
	
	for clientIdx := 0; clientIdx < numClients; clientIdx++ {
		wg.Add(1)
		go func(c *client.Client) {
			defer wg.Done()
			ctx := context.Background()
			
			for i := 0; i < opsPerClient; i++ {
				opNum := atomic.AddInt64(&opCounter, 1)
				
				// Determine operation type based on read ratio
				isRead := readRatio > 0 && (float64(opNum%100)/100.0) < readRatio
				
				if isRead && len(storedHashes) > 0 {
					// GET operation
					hashIndex := int(opNum) % len(storedHashes)
					hash := storedHashes[hashIndex]
					
					_, err := c.Get(ctx, hash)
					if err != nil {
						b.Errorf("GET operation failed: %v", err)
						return
					}
				} else {
					// SET operation
					payload := make([]byte, needle.PayloadLength)
					for j := range payload {
						payload[j] = byte((int(opNum) + j) % 256)
					}
					
					testNeedle, err := needle.New(payload)
					if err != nil {
						b.Errorf("Failed to create test needle: %v", err)
						return
					}
					
					err = c.Set(ctx, testNeedle)
					if err != nil {
						b.Errorf("SET operation failed: %v", err)
						return
					}
				}
			}
		}(clients[clientIdx])
	}
	
	wg.Wait()
	
	duration := time.Since(start)
	totalOps := atomic.LoadInt64(&opCounter)
	opsPerSec := float64(totalOps) / duration.Seconds()
	
	b.ReportMetric(opsPerSec, "ops/sec")
}

// measureThroughput measures sustained throughput with given number of clients
func measureThroughput(b *testing.B, serverAddr string, numClients int) {
	// Create clients
	clients := make([]*client.Client, numClients)
	for i := 0; i < numClients; i++ {
		config := client.DefaultConfig(serverAddr)
		config.MaxConnections = 10
		c, err := client.New(config)
		if err != nil {
			b.Fatalf("Failed to create client %d: %v", i, err)
		}
		defer c.Close()
		clients[i] = c
	}
	
	// Create test needle
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	testNeedle, err := needle.New(payload)
	if err != nil {
		b.Fatalf("Failed to create test needle: %v", err)
	}
	
	var totalOps int64
	
	b.ResetTimer()
	
	// Run for fixed duration to measure sustained throughput
	duration := time.Second * 10
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	var wg sync.WaitGroup
	start := time.Now()
	
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(c *client.Client) {
			defer wg.Done()
			
			for {
				select {
				case <-ctx.Done():
					return
				default:
					err := c.Set(context.Background(), testNeedle)
					if err != nil {
						b.Errorf("SET operation failed: %v", err)
						return
					}
					atomic.AddInt64(&totalOps, 1)
				}
			}
		}(clients[i])
	}
	
	wg.Wait()
	actualDuration := time.Since(start)
	
	opsPerSec := float64(totalOps) / actualDuration.Seconds()
	b.ReportMetric(opsPerSec, "ops/sec")
	b.ReportMetric(float64(totalOps), "total_ops")
}

// startE2EServer starts a server optimized for end-to-end benchmarks
func startE2EServer(b *testing.B) string {
	// Create storage backend with large capacity
	ctx := context.Background()
	storage := memory.New(ctx, time.Hour, 10000000) // 10M items
	return startServerWithStorage(b, storage)
}

// startServerWithStorage starts a server with the given storage backend
func startServerWithStorage(b *testing.B, storage storage.GetSetCloser) string {
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
			b.Logf("E2E benchmark server stopped: %v", err)
		}
	}()
	
	// Give server time to start
	time.Sleep(50 * time.Millisecond)
	
	// Clean up when benchmark completes
	b.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		storage.Close()
	})
	
	return addr
}