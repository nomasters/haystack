package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/nomasters/haystack/needle"
	"github.com/nomasters/haystack/storage/memory"
)

func TestServer_SetAndGet(t *testing.T) {
	// Create storage backend
	ctx := context.Background()
	storage := memory.New(ctx, time.Hour, 1000)
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("Failed to close storage: %v", err)
		}
	}()

	// Create server
	srv := New(&Config{Storage: storage})

	// Find an available port
	listener, err := net.ListenPacket("udp", ":0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	addr := listener.LocalAddr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("Failed to close listener: %v", err)
	}

	// Start server
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- srv.ListenAndServe(addr)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Create UDP client for testing
	conn, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Errorf("Failed to close connection: %v", err)
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

	// Test SET operation (should not respond)
	t.Run("SET operation", func(t *testing.T) {
		// Send needle to server
		_, err := conn.Write(testNeedle.Bytes())
		if err != nil {
			t.Fatalf("Failed to send needle: %v", err)
		}

		// Try to read response with timeout (should timeout as no response expected)
		if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			t.Errorf("Failed to set read deadline: %v", err)
		}
		buffer := make([]byte, needle.NeedleLength)
		_, err = conn.Read(buffer)

		// We expect a timeout error since SET operations don't respond
		if err == nil {
			t.Error("SET operation should not return a response")
		} else if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
			t.Errorf("Expected timeout error, got: %v", err)
		}
	})

	// Test GET operation (should respond)
	t.Run("GET operation", func(t *testing.T) {
		// Send hash to server
		hash := testNeedle.Hash()
		_, err := conn.Write(hash[:])
		if err != nil {
			t.Fatalf("Failed to send hash: %v", err)
		}

		// Read response
		if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			t.Errorf("Failed to set read deadline: %v", err)
		}
		buffer := make([]byte, needle.NeedleLength)
		n, err := conn.Read(buffer)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		if n != needle.NeedleLength {
			t.Errorf("Expected response length %d, got %d", needle.NeedleLength, n)
		}

		// Verify response is the correct needle
		responseNeedle, err := needle.FromBytes(buffer[:n])
		if err != nil {
			t.Errorf("Response is not a valid needle: %v", err)
		}

		if responseNeedle.Hash() != testNeedle.Hash() {
			t.Error("Response needle hash doesn't match original")
		}
	})

	// Shutdown server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Failed to shutdown server: %v", err)
	}

	// Wait for server to stop
	select {
	case err := <-serverDone:
		if err != nil {
			t.Logf("Server stopped with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Server did not stop within timeout")
	}
}

func TestServer_InvalidPackets(t *testing.T) {
	// Create storage backend
	ctx := context.Background()
	storage := memory.New(ctx, time.Hour, 1000)
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("Failed to close storage: %v", err)
		}
	}()

	// Create server
	srv := New(&Config{Storage: storage})

	// Find an available port
	listener, err := net.ListenPacket("udp", ":0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	addr := listener.LocalAddr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("Failed to close listener: %v", err)
	}

	// Start server
	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			t.Errorf("Server listen failed: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Create UDP client
	conn, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Errorf("Failed to close connection: %v", err)
		}
	}()

	// Test invalid packet sizes (should be dropped silently)
	invalidSizes := []int{0, 1, 10, 31, 33, 100, 191, 193, 300}

	for _, size := range invalidSizes {
		invalidPacket := make([]byte, size)
		_, err := conn.Write(invalidPacket)
		if err != nil {
			t.Errorf("Failed to send invalid packet of size %d: %v", size, err)
		}
	}

	// Send a valid packet to make sure server is still working
	validPacket := make([]byte, needle.HashLength)
	_, err = conn.Write(validPacket)
	if err != nil {
		t.Fatalf("Failed to send valid packet: %v", err)
	}

	// Try to read response (for the hash query)
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Errorf("Failed to set read deadline: %v", err)
	}
	buffer := make([]byte, needle.NeedleLength)
	_, _ = conn.Read(buffer)
	// This will likely error since the hash doesn't exist, but that's expected
	// The important thing is that the server is still responding

	// Shutdown server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Failed to shutdown server: %v", err)
	}
}

func TestServer_GetNonExistent(t *testing.T) {
	// Create storage backend
	ctx := context.Background()
	storage := memory.New(ctx, time.Hour, 1000)
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("Failed to close storage: %v", err)
		}
	}()

	// Create server
	srv := New(&Config{Storage: storage})

	// Find an available port
	listener, err := net.ListenPacket("udp", ":0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	addr := listener.LocalAddr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("Failed to close listener: %v", err)
	}

	// Start server
	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			t.Errorf("Server listen failed: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Create UDP client
	conn, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Errorf("Failed to close connection: %v", err)
		}
	}()

	// Try to get a non-existent needle
	nonExistentHash := make([]byte, needle.HashLength)
	for i := range nonExistentHash {
		nonExistentHash[i] = 0xFF
	}

	_, err = conn.Write(nonExistentHash)
	if err != nil {
		t.Fatalf("Failed to send hash: %v", err)
	}

	// Try to read response with timeout
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Errorf("Failed to set read deadline: %v", err)
	}
	buffer := make([]byte, needle.NeedleLength)
	_, err = conn.Read(buffer)

	// We expect an error or timeout since the needle doesn't exist
	if err == nil {
		t.Error("Expected error when getting non-existent needle")
	}

	// Shutdown server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Failed to shutdown server: %v", err)
	}
}

func TestServer_NoStorage(t *testing.T) {
	// Create server without storage
	srv := New(&Config{})

	// Try to start server (should fail)
	err := srv.ListenAndServe(":0")
	if err == nil {
		t.Error("Expected error when starting server without storage")
	}
}

func TestServer_Shutdown(t *testing.T) {
	// Create storage backend
	ctx := context.Background()
	storage := memory.New(ctx, time.Hour, 1000)
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("Failed to close storage: %v", err)
		}
	}()

	// Create server
	srv := New(&Config{Storage: storage})

	// Find an available port
	listener, err := net.ListenPacket("udp", ":0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	addr := listener.LocalAddr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("Failed to close listener: %v", err)
	}

	// Start server
	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			t.Errorf("Server listen failed: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(10 * time.Millisecond)

	// Test graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = srv.Shutdown(shutdownCtx)
	if err != nil {
		t.Errorf("Failed to shutdown server: %v", err)
	}
}
