package server

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/nomasters/haystack/logger"
	"github.com/nomasters/haystack/needle"
	"github.com/nomasters/haystack/storage/memory"
)

func BenchmarkServer_ConcurrentGET(b *testing.B) {
	// Setup server
	ctx := context.Background()
	storage := memory.New(ctx, 5*time.Minute, 10000)
	config := &Config{
		Storage: storage,
		Logger:  logger.NewNoOp(),
	}

	server := New(config)

	// Start server
	go func() {
		if err := server.ListenAndServe("127.0.0.1:0"); err != nil {
			b.Logf("Server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Get actual address
	addr := server.conn.LocalAddr()

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	// Pre-populate with test data
	testNeedles := make([]*needle.Needle, 100)
	for i := range testNeedles {
		data := make([]byte, 160)
		data[0] = byte(i)
		n, _ := needle.New(data)
		testNeedles[i] = n
		storage.Set(n)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		conn, err := net.Dial("udp", addr.String())
		if err != nil {
			b.Fatal(err)
		}
		defer conn.Close()

		buf := make([]byte, needle.NeedleLength)
		i := 0

		for pb.Next() {
			// Send GET request
			hash := testNeedles[i%len(testNeedles)].Hash()
			if _, err := conn.Write(hash[:]); err != nil {
				b.Fatal(err)
			}

			// Read response
			conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			if _, err := conn.Read(buf); err != nil {
				// Timeout is expected for non-blocking responses
				if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
					b.Fatal(err)
				}
			}

			i++
		}
	})
}

func BenchmarkServer_MixedLoad(b *testing.B) {
	// Setup server
	ctx := context.Background()
	storage := memory.New(ctx, 5*time.Minute, 10000)
	config := &Config{
		Storage: storage,
		Logger:  logger.NewNoOp(),
	}

	server := New(config)

	// Start server
	go func() {
		if err := server.ListenAndServe("127.0.0.1:0"); err != nil {
			b.Logf("Server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Get actual address
	addr := server.conn.LocalAddr()

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	// Pre-populate with test data
	testNeedles := make([]*needle.Needle, 100)
	for i := range testNeedles {
		data := make([]byte, 160)
		data[0] = byte(i)
		n, _ := needle.New(data)
		testNeedles[i] = n
	}

	b.ResetTimer()

	// Run mixed SET and GET operations
	var wg sync.WaitGroup

	// SET workers
	for w := 0; w < 5; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.Dial("udp", addr.String())
			if err != nil {
				b.Error(err)
				return
			}
			defer conn.Close()

			for i := 0; i < b.N/10; i++ {
				needle := testNeedles[i%len(testNeedles)]
				conn.Write(needle.Bytes())
			}
		}()
	}

	// GET workers
	for w := 0; w < 5; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.Dial("udp", addr.String())
			if err != nil {
				b.Error(err)
				return
			}
			defer conn.Close()

			buf := make([]byte, needle.NeedleLength)
			for i := 0; i < b.N/10; i++ {
				hash := testNeedles[i%len(testNeedles)].Hash()
				conn.Write(hash[:])

				conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
				conn.Read(buf) // Ignore errors, we're testing throughput
			}
		}()
	}

	wg.Wait()
}
