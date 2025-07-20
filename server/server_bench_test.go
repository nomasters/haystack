package server

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nomasters/haystack/needle"
	"github.com/nomasters/haystack/storage/memory"
)

// BenchmarkServer_SET benchmarks write operations to the server
func BenchmarkServer_SET(b *testing.B) {
	// Create storage backend
	ctx := context.Background()
	storage := memory.New(ctx, time.Hour, 100000)
	defer func() {
		if err := storage.Close(); err != nil {
			b.Fatalf("Failed to close storage: %v", err)
		}
	}()
	
	// Create server
	srv := New(&Config{Storage: storage})
	
	// Find an available port
	listener, err := net.ListenPacket("udp", ":0")
	if err != nil {
		b.Fatalf("Failed to find available port: %v", err)
	}
	addr := listener.LocalAddr().String()
	if err := listener.Close(); err != nil {
		b.Fatalf("Failed to close listener: %v", err)
	}
	
	// Start server
	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			b.Logf("Server error: %v", err)
		}
	}()
	
	// Give server time to start
	time.Sleep(10 * time.Millisecond)
	
	// Create test needle
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	testNeedle, err := needle.New(payload)
	if err != nil {
		b.Fatalf("Failed to create test needle: %v", err)
	}
	
	// Create UDP connection
	conn, err := net.Dial("udp", addr)
	if err != nil {
		b.Fatalf("Failed to connect to server: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			b.Errorf("Failed to close connection: %v", err)
		}
	}()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Benchmark SET operations
	for i := 0; i < b.N; i++ {
		_, err := conn.Write(testNeedle.Bytes())
		if err != nil {
			b.Fatalf("Failed to write needle: %v", err)
		}
	}
	
	// Cleanup
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		b.Errorf("Failed to shutdown server: %v", err)
	}
}

// BenchmarkServer_GET benchmarks read operations from the server
func BenchmarkServer_GET(b *testing.B) {
	// Create storage backend
	ctx := context.Background()
	storage := memory.New(ctx, time.Hour, 100000)
	defer func() {
		if err := storage.Close(); err != nil {
			b.Fatalf("Failed to close storage: %v", err)
		}
	}()
	
	// Create server
	srv := New(&Config{Storage: storage})
	
	// Find an available port
	listener, err := net.ListenPacket("udp", ":0")
	if err != nil {
		b.Fatalf("Failed to find available port: %v", err)
	}
	addr := listener.LocalAddr().String()
	if err := listener.Close(); err != nil {
		b.Fatalf("Failed to close listener: %v", err)
	}
	
	// Start server
	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			b.Logf("Server error: %v", err)
		}
	}()
	
	// Give server time to start
	time.Sleep(10 * time.Millisecond)
	
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
	conn, err := net.Dial("udp", addr)
	if err != nil {
		b.Fatalf("Failed to connect to server: %v", err)
	}
	
	_, err = conn.Write(testNeedle.Bytes())
	if err != nil {
		b.Fatalf("Failed to store test needle: %v", err)
	}
	
	// Give storage time to complete
	time.Sleep(10 * time.Millisecond)
	
	hash := testNeedle.Hash()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Benchmark GET operations
	for i := 0; i < b.N; i++ {
		// Send hash query
		_, err := conn.Write(hash[:])
		if err != nil {
			b.Fatalf("Failed to write hash: %v", err)
		}
		
		// Read response
		buffer := make([]byte, needle.NeedleLength)
		_, err = conn.Read(buffer)
		if err != nil {
			b.Fatalf("Failed to read response: %v", err)
		}
	}
	
	if err := conn.Close(); err != nil {
		b.Errorf("Failed to close connection: %v", err)
	}
	
	// Cleanup
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		b.Errorf("Failed to shutdown server: %v", err)
	}
}

// BenchmarkServer_Concurrent_SET benchmarks concurrent write operations
func BenchmarkServer_Concurrent_SET(b *testing.B) {
	// Create storage backend
	ctx := context.Background()
	storage := memory.New(ctx, time.Hour, 100000)
	defer func() {
		if err := storage.Close(); err != nil {
			b.Fatalf("Failed to close storage: %v", err)
		}
	}()
	
	// Create server
	srv := New(&Config{Storage: storage})
	
	// Find an available port
	listener, err := net.ListenPacket("udp", ":0")
	if err != nil {
		b.Fatalf("Failed to find available port: %v", err)
	}
	addr := listener.LocalAddr().String()
	if err := listener.Close(); err != nil {
		b.Fatalf("Failed to close listener: %v", err)
	}
	
	// Start server
	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			b.Logf("Server error: %v", err)
		}
	}()
	
	// Give server time to start
	time.Sleep(10 * time.Millisecond)
	
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
	
	// Run concurrent SET operations
	b.RunParallel(func(pb *testing.PB) {
		// Each goroutine gets its own connection
		conn, err := net.Dial("udp", addr)
		if err != nil {
			b.Errorf("Failed to connect to server: %v", err)
			return
		}
		defer func() {
		if err := conn.Close(); err != nil {
			b.Errorf("Failed to close connection: %v", err)
		}
	}()
		
		for pb.Next() {
			_, err := conn.Write(testNeedle.Bytes())
			if err != nil {
				b.Errorf("Failed to write needle: %v", err)
				return
			}
		}
	})
	
	// Cleanup
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		b.Errorf("Failed to shutdown server: %v", err)
	}
}

// BenchmarkServer_Concurrent_GET benchmarks concurrent read operations
func BenchmarkServer_Concurrent_GET(b *testing.B) {
	// Create storage backend
	ctx := context.Background()
	storage := memory.New(ctx, time.Hour, 100000)
	defer func() {
		if err := storage.Close(); err != nil {
			b.Fatalf("Failed to close storage: %v", err)
		}
	}()
	
	// Create server
	srv := New(&Config{Storage: storage})
	
	// Find an available port
	listener, err := net.ListenPacket("udp", ":0")
	if err != nil {
		b.Fatalf("Failed to find available port: %v", err)
	}
	addr := listener.LocalAddr().String()
	if err := listener.Close(); err != nil {
		b.Fatalf("Failed to close listener: %v", err)
	}
	
	// Start server
	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			b.Logf("Server error: %v", err)
		}
	}()
	
	// Give server time to start
	time.Sleep(10 * time.Millisecond)
	
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
	setupConn, err := net.Dial("udp", addr)
	if err != nil {
		b.Fatalf("Failed to connect to server: %v", err)
	}
	
	_, err = setupConn.Write(testNeedle.Bytes())
	if err != nil {
		b.Fatalf("Failed to store test needle: %v", err)
	}
	if err := setupConn.Close(); err != nil {
		b.Errorf("Failed to close setup connection: %v", err)
	}
	
	// Give storage time to complete
	time.Sleep(10 * time.Millisecond)
	
	hash := testNeedle.Hash()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Run concurrent GET operations
	b.RunParallel(func(pb *testing.PB) {
		// Each goroutine gets its own connection
		conn, err := net.Dial("udp", addr)
		if err != nil {
			b.Errorf("Failed to connect to server: %v", err)
			return
		}
		defer func() {
		if err := conn.Close(); err != nil {
			b.Errorf("Failed to close connection: %v", err)
		}
	}()
		
		for pb.Next() {
			// Send hash query
			_, err := conn.Write(hash[:])
			if err != nil {
				b.Errorf("Failed to write hash: %v", err)
				return
			}
			
			// Read response
			buffer := make([]byte, needle.NeedleLength)
			_, err = conn.Read(buffer)
			if err != nil {
				b.Errorf("Failed to read response: %v", err)
				return
			}
		}
	})
	
	// Cleanup
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		b.Errorf("Failed to shutdown server: %v", err)
	}
}

// BenchmarkServer_Mixed_Operations benchmarks mixed read/write workload
func BenchmarkServer_Mixed_Operations(b *testing.B) {
	// Create storage backend
	ctx := context.Background()
	storage := memory.New(ctx, time.Hour, 100000)
	defer func() {
		if err := storage.Close(); err != nil {
			b.Fatalf("Failed to close storage: %v", err)
		}
	}()
	
	// Create server
	srv := New(&Config{Storage: storage})
	
	// Find an available port
	listener, err := net.ListenPacket("udp", ":0")
	if err != nil {
		b.Fatalf("Failed to find available port: %v", err)
	}
	addr := listener.LocalAddr().String()
	if err := listener.Close(); err != nil {
		b.Fatalf("Failed to close listener: %v", err)
	}
	
	// Start server
	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			b.Logf("Server error: %v", err)
		}
	}()
	
	// Give server time to start
	time.Sleep(10 * time.Millisecond)
	
	// Pre-populate with some data
	setupConn, err := net.Dial("udp", addr)
	if err != nil {
		b.Fatalf("Failed to connect to server: %v", err)
	}
	
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
		
		_, err = setupConn.Write(testNeedle.Bytes())
		if err != nil {
			b.Fatalf("Failed to store test needle: %v", err)
		}
		
		storedHashes = append(storedHashes, testNeedle.Hash())
	}
	if err := setupConn.Close(); err != nil {
		b.Errorf("Failed to close setup connection: %v", err)
	}
	
	// Give storage time to complete
	time.Sleep(100 * time.Millisecond)
	
	var opCounter int64
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Run mixed operations (70% reads, 30% writes)
	b.RunParallel(func(pb *testing.PB) {
		conn, err := net.Dial("udp", addr)
		if err != nil {
			b.Errorf("Failed to connect to server: %v", err)
			return
		}
		defer func() {
		if err := conn.Close(); err != nil {
			b.Errorf("Failed to close connection: %v", err)
		}
	}()
		
		for pb.Next() {
			opNum := atomic.AddInt64(&opCounter, 1)
			
			if opNum%10 < 7 { // 70% reads
				// GET operation
				hashIndex := int(opNum) % len(storedHashes)
				hash := storedHashes[hashIndex]
				
				_, err := conn.Write(hash[:])
				if err != nil {
					b.Errorf("Failed to write hash: %v", err)
					return
				}
				
				buffer := make([]byte, needle.NeedleLength)
				_, err = conn.Read(buffer)
				if err != nil {
					b.Errorf("Failed to read response: %v", err)
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
				
				_, err = conn.Write(testNeedle.Bytes())
				if err != nil {
					b.Errorf("Failed to write needle: %v", err)
					return
				}
			}
		}
	})
	
	// Cleanup
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		b.Errorf("Failed to shutdown server: %v", err)
	}
}

// BenchmarkServer_Throughput measures overall throughput with multiple connections
func BenchmarkServer_Throughput(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping throughput benchmark in short mode")
	}
	
	// Create storage backend
	ctx := context.Background()
	storage := memory.New(ctx, time.Hour, 100000)
	defer func() {
		if err := storage.Close(); err != nil {
			b.Fatalf("Failed to close storage: %v", err)
		}
	}()
	
	// Create server
	srv := New(&Config{Storage: storage})
	
	// Find an available port
	listener, err := net.ListenPacket("udp", ":0")
	if err != nil {
		b.Fatalf("Failed to find available port: %v", err)
	}
	addr := listener.LocalAddr().String()
	if err := listener.Close(); err != nil {
		b.Fatalf("Failed to close listener: %v", err)
	}
	
	// Start server
	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			b.Logf("Server error: %v", err)
		}
	}()
	
	// Give server time to start
	time.Sleep(10 * time.Millisecond)
	
	// Test different numbers of concurrent connections
	for _, numConns := range []int{1, 5, 10, 25, 50, 100} {
		b.Run(fmt.Sprintf("connections_%d", numConns), func(b *testing.B) {
			var wg sync.WaitGroup
			opsPerConn := b.N / numConns
			if opsPerConn == 0 {
				opsPerConn = 1
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
			
			b.ResetTimer()
			
			for i := 0; i < numConns; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					
					conn, err := net.Dial("udp", addr)
					if err != nil {
						b.Errorf("Failed to connect to server: %v", err)
						return
					}
					defer func() {
		if err := conn.Close(); err != nil {
			b.Errorf("Failed to close connection: %v", err)
		}
	}()
					
					for j := 0; j < opsPerConn; j++ {
						_, err := conn.Write(testNeedle.Bytes())
						if err != nil {
							b.Errorf("Failed to write needle: %v", err)
							return
						}
					}
				}()
			}
			
			wg.Wait()
		})
	}
	
	// Cleanup
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		b.Errorf("Failed to shutdown server: %v", err)
	}
}