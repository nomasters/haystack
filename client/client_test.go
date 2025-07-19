package client

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/nomasters/haystack/needle"
	"github.com/nomasters/haystack/server"
	"github.com/nomasters/haystack/storage/memory"
)

func TestClient_SetAndGet(t *testing.T) {
	// Start a test server
	serverAddr := startTestServer(t)
	
	// Create client
	config := DefaultConfig(serverAddr)
	config.MaxConnections = 2
	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx := context.Background()
	
	// Create test data
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	
	testNeedle, err := needle.New(payload)
	if err != nil {
		t.Fatalf("Failed to create test needle: %v", err)
	}
	
	// Test SET operation
	t.Run("SET operation", func(t *testing.T) {
		err := client.Set(ctx, testNeedle)
		if err != nil {
			t.Errorf("SET operation failed: %v", err)
		}
	})
	
	// Test GET operation
	t.Run("GET operation", func(t *testing.T) {
		hash := testNeedle.Hash()
		retrievedNeedle, err := client.Get(ctx, hash)
		if err != nil {
			t.Errorf("GET operation failed: %v", err)
		}
		
		if retrievedNeedle.Hash() != testNeedle.Hash() {
			t.Error("Retrieved needle hash doesn't match original")
		}
		
		// Compare payloads
		originalPayload := testNeedle.Payload()
		retrievedPayload := retrievedNeedle.Payload()
		
		for i := range originalPayload {
			if originalPayload[i] != retrievedPayload[i] {
				t.Errorf("Payload mismatch at index %d: expected %d, got %d", i, originalPayload[i], retrievedPayload[i])
				break
			}
		}
	})
}

func TestClient_GetNonExistent(t *testing.T) {
	// Start a test server
	serverAddr := startTestServer(t)
	
	// Create client
	config := DefaultConfig(serverAddr)
	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx := context.Background()
	
	// Try to get a non-existent needle
	var nonExistentHash needle.Hash
	for i := range nonExistentHash {
		nonExistentHash[i] = 0xFF
	}
	
	_, err = client.Get(ctx, nonExistentHash)
	if err == nil {
		t.Error("Expected error when getting non-existent needle")
	}
}

func TestClient_InvalidData(t *testing.T) {
	// Start a test server
	serverAddr := startTestServer(t)
	
	// Create client
	config := DefaultConfig(serverAddr)
	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx := context.Background()
	
	t.Run("Invalid SET data", func(t *testing.T) {
		// Try to set invalid data
		invalidData := make([]byte, 100) // Wrong size
		err := client.SetBytes(ctx, invalidData)
		if err == nil {
			t.Error("Expected error when setting invalid data")
		}
	})
	
	t.Run("Invalid GET hash", func(t *testing.T) {
		// Try to get with invalid hash
		invalidHash := make([]byte, 10) // Wrong size
		_, err := client.GetBytes(ctx, invalidHash)
		if err == nil {
			t.Error("Expected error when getting with invalid hash")
		}
	})
}

func TestClient_Timeouts(t *testing.T) {
	// Start a test server
	serverAddr := startTestServer(t)
	
	// Create client with very short timeouts
	config := DefaultConfig(serverAddr)
	config.ReadTimeout = 1 * time.Nanosecond  // Guaranteed to timeout
	config.WriteTimeout = 1 * time.Nanosecond
	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx := context.Background()
	
	// Create test data
	payload := make([]byte, needle.PayloadLength)
	testNeedle, err := needle.New(payload)
	if err != nil {
		t.Fatalf("Failed to create test needle: %v", err)
	}
	
	t.Run("SET timeout", func(t *testing.T) {
		err := client.Set(ctx, testNeedle)
		if err == nil {
			t.Error("Expected timeout error for SET operation")
		}
	})
	
	t.Run("GET timeout", func(t *testing.T) {
		hash := testNeedle.Hash()
		_, err := client.Get(ctx, hash)
		if err == nil {
			t.Error("Expected timeout error for GET operation")
		}
	})
}

func TestClient_ConnectionPool(t *testing.T) {
	// Start a test server
	serverAddr := startTestServer(t)
	
	// Create client with limited pool size
	config := DefaultConfig(serverAddr)
	config.MaxConnections = 2
	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx := context.Background()
	
	// Create test data
	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	
	testNeedle, err := needle.New(payload)
	if err != nil {
		t.Fatalf("Failed to create test needle: %v", err)
	}
	
	// Perform multiple operations to test pool reuse
	for i := range 5 {
		// SET operation
		err := client.Set(ctx, testNeedle)
		if err != nil {
			t.Errorf("SET operation %d failed: %v", i, err)
		}
		
		// GET operation
		hash := testNeedle.Hash()
		_, err = client.Get(ctx, hash)
		if err != nil {
			t.Errorf("GET operation %d failed: %v", i, err)
		}
	}
	
	// Check pool stats
	stats := client.Stats()
	if stats.Total == 0 {
		t.Error("Expected some connections to be created")
	}
	
	t.Logf("Pool stats: Active=%d, Idle=%d, Total=%d", stats.Active, stats.Idle, stats.Total)
}

func TestClient_ContextCancellation(t *testing.T) {
	// Start a test server
	serverAddr := startTestServer(t)
	
	// Create client
	config := DefaultConfig(serverAddr)
	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	// Create test data
	payload := make([]byte, needle.PayloadLength)
	testNeedle, err := needle.New(payload)
	if err != nil {
		t.Fatalf("Failed to create test needle: %v", err)
	}
	
	t.Run("Cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		err := client.Set(ctx, testNeedle)
		// Note: UDP operations might still succeed even with cancelled context
		// because they're fire-and-forget. This test mainly ensures no panic.
		t.Logf("SET with cancelled context: %v", err)
	})
	
	t.Run("Context with deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		
		// First set the needle
		err := client.Set(context.Background(), testNeedle)
		if err != nil {
			t.Fatalf("Failed to set needle: %v", err)
		}
		
		// Then try to get it with short deadline
		hash := testNeedle.Hash()
		_, err = client.Get(ctx, hash)
		// This may or may not timeout depending on timing, but shouldn't panic
		t.Logf("GET with deadline: %v", err)
	})
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig("localhost:1337")
	
	if config.Address != "localhost:1337" {
		t.Errorf("Expected address 'localhost:1337', got '%s'", config.Address)
	}
	
	if config.MaxConnections == 0 {
		t.Error("Expected non-zero MaxConnections")
	}
	
	if config.ReadTimeout == 0 {
		t.Error("Expected non-zero ReadTimeout")
	}
	
	if config.WriteTimeout == 0 {
		t.Error("Expected non-zero WriteTimeout")
	}
	
	if config.IdleTimeout == 0 {
		t.Error("Expected non-zero IdleTimeout")
	}
}

func TestNew_InvalidConfig(t *testing.T) {
	// Test with empty address
	config := &Config{}
	_, err := New(config)
	if err == nil {
		t.Error("Expected error for empty address")
	}
}

// startTestServer starts a Haystack server for testing and returns its address.
func startTestServer(t *testing.T) string {
	// Create storage backend
	ctx := context.Background()
	storage := memory.New(ctx, time.Hour, 1000)
	
	// Create UDP server
	srv := server.New(&server.Config{Storage: storage})
	
	// Find available port
	listener, err := net.ListenPacket("udp", ":0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	addr := listener.LocalAddr().String()
	listener.Close()
	
	// Start server
	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			t.Logf("Test server stopped: %v", err)
		}
	}()
	
	// Give server time to start
	time.Sleep(10 * time.Millisecond)
	
	// Clean up when test completes
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		storage.Close()
	})
	
	return addr
}