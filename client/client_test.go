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
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()
	
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
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()
	
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
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()
	
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
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()
	
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
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()
	
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
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()
	
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
	if err := listener.Close(); err != nil {
		t.Fatalf("Failed to close listener: %v", err)
	}
	
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
		if err := srv.Shutdown(ctx); err != nil {
			t.Errorf("Failed to shutdown server: %v", err)
		}
		if err := storage.Close(); err != nil {
			t.Errorf("Failed to close storage: %v", err)
		}
	})
	
	return addr
}

func TestConnectionPool_Cleanup(t *testing.T) {
	// Start a test server
	serverAddr := startTestServer(t)
	
	// Create client with very short idle timeout to trigger cleanup
	config := DefaultConfig(serverAddr)
	config.MaxConnections = 3
	config.IdleTimeout = 50 * time.Millisecond // Very short idle timeout
	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()
	
	ctx := context.Background()
	
	// Create test data
	payload := make([]byte, needle.PayloadLength)
	testNeedle, err := needle.New(payload)
	if err != nil {
		t.Fatalf("Failed to create test needle: %v", err)
	}
	
	// Perform several operations to create connections
	for i := 0; i < 5; i++ {
		err := client.Set(ctx, testNeedle)
		if err != nil {
			t.Errorf("SET operation %d failed: %v", i, err)
		}
	}
	
	// Check that we have some connections
	stats := client.Stats()
	t.Logf("Before cleanup: Active=%d, Idle=%d, Total=%d", stats.Active, stats.Idle, stats.Total)
	
	// Wait for cleanup to trigger (idle timeout + cleanup interval)
	time.Sleep(100 * time.Millisecond)
	
	// The cleanup goroutine should have cleaned up idle connections
	stats = client.Stats()
	t.Logf("After cleanup: Active=%d, Idle=%d, Total=%d", stats.Active, stats.Idle, stats.Total)
}

func TestConnectionPool_EdgeCases(t *testing.T) {
	serverAddr := startTestServer(t)
	
	t.Run("Pool_GetFromClosedPool", func(t *testing.T) {
		config := DefaultConfig(serverAddr)
		client, err := New(config)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		
		// Close the client first
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
		
		// Try to use client after closing
		ctx := context.Background()
		payload := make([]byte, needle.PayloadLength)
		err = client.SetBytes(ctx, payload)
		if err == nil {
			t.Error("Expected error when using closed client")
		}
	})
	
	t.Run("Pool_BadConnections", func(t *testing.T) {
		config := DefaultConfig(serverAddr)
		config.MaxConnections = 2
		client, err := New(config)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer func() {
			if err := client.Close(); err != nil {
				t.Errorf("Failed to close client: %v", err)
			}
		}()
		
		// Fill the pool with connections
		ctx := context.Background()
		payload := make([]byte, needle.NeedleLength)
		
		for i := 0; i < 3; i++ {
			err := client.SetBytes(ctx, payload)
			if err != nil {
				t.Errorf("SET operation %d failed: %v", i, err)
			}
		}
		
		// Verify pool stats
		stats := client.Stats()
		t.Logf("Pool stats after operations: Active=%d, Idle=%d, Total=%d", stats.Active, stats.Idle, stats.Total)
	})
	
	t.Run("Pool_OverflowConnections", func(t *testing.T) {
		config := DefaultConfig(serverAddr)
		config.MaxConnections = 1 // Very small pool
		client, err := New(config)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer func() {
			if err := client.Close(); err != nil {
				t.Errorf("Failed to close client: %v", err)
			}
		}()
		
		ctx := context.Background()
		payload := make([]byte, needle.NeedleLength)
		
		// Perform more operations than pool size to test overflow handling
		for i := 0; i < 5; i++ {
			err := client.SetBytes(ctx, payload)
			if err != nil {
				t.Errorf("SET operation %d failed: %v", i, err)
			}
		}
		
		stats := client.Stats()
		t.Logf("Small pool stats: Active=%d, Idle=%d, Total=%d", stats.Active, stats.Idle, stats.Total)
	})
}

func TestNew_ConfigEdgeCases(t *testing.T) {
	t.Run("ConfigWithDefaults", func(t *testing.T) {
		config := &Config{
			Address: "localhost:1337",
			// Leave all other fields as zero values to test default application
		}
		
		client, err := New(config)
		if err != nil {
			t.Errorf("Failed to create client with minimal config: %v", err)
		} else {
			if err := client.Close(); err != nil {
				t.Errorf("Failed to close client: %v", err)
			}
		}
	})
	
	t.Run("ConfigWithCustomLogger", func(t *testing.T) {
		// Use a mock logger to test logger path
		config := DefaultConfig("localhost:1337")
		config.Logger = &mockLogger{}
		
		client, err := New(config)
		if err != nil {
			t.Errorf("Failed to create client with custom logger: %v", err)
		} else {
			if err := client.Close(); err != nil {
				t.Errorf("Failed to close client: %v", err)
			}
		}
	})
	
	t.Run("ConfigWithZeroValues", func(t *testing.T) {
		config := &Config{
			Address:        "localhost:1337",
			MaxConnections: 0, // Should get default
			ReadTimeout:    0, // Should get default
			WriteTimeout:   0, // Should get default
			IdleTimeout:    0, // Should get default
		}
		
		client, err := New(config)
		if err != nil {
			t.Errorf("Failed to create client with zero values: %v", err)
		} else {
			if err := client.Close(); err != nil {
				t.Errorf("Failed to close client: %v", err)
			}
		}
	})
}

func TestSetBytes_ContextDeadlines(t *testing.T) {
	serverAddr := startTestServer(t)
	
	config := DefaultConfig(serverAddr)
	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()
	
	payload := make([]byte, needle.NeedleLength)
	
	t.Run("ContextWithDeadline", func(t *testing.T) {
		// Test with context that has a deadline
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		
		err := client.SetBytes(ctx, payload)
		if err != nil {
			t.Errorf("SetBytes with deadline failed: %v", err)
		}
	})
	
	t.Run("ContextWithPastDeadline", func(t *testing.T) {
		// Test with context that has already expired
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		
		// Wait for context to expire
		time.Sleep(10 * time.Millisecond)
		
		err := client.SetBytes(ctx, payload)
		if err == nil {
			t.Error("Expected error with expired context")
		}
	})
}

func TestGetBytes_ContextDeadlines(t *testing.T) {
	serverAddr := startTestServer(t)
	
	config := DefaultConfig(serverAddr)
	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()
	
	// First set a needle
	ctx := context.Background()
	payload := make([]byte, needle.PayloadLength)
	testNeedle, err := needle.New(payload)
	if err != nil {
		t.Fatalf("Failed to create test needle: %v", err)
	}
	
	err = client.Set(ctx, testNeedle)
	if err != nil {
		t.Fatalf("Failed to set needle: %v", err)
	}
	
	hash := testNeedle.Hash()
	
	t.Run("GetBytesWithDeadline", func(t *testing.T) {
		// Test with context that has a deadline
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		
		_, err := client.GetBytes(ctx, hash[:])
		if err != nil {
			t.Errorf("GetBytes with deadline failed: %v", err)
		}
	})
	
	t.Run("GetBytesWithPastDeadline", func(t *testing.T) {
		// Test with context that has already expired
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		
		// Wait for context to expire
		time.Sleep(10 * time.Millisecond)
		
		_, err := client.GetBytes(ctx, hash[:])
		if err == nil {
			t.Error("Expected error with expired context")
		}
	})
}

func TestConnectionPool_StaleConnections(t *testing.T) {
	serverAddr := startTestServer(t)
	
	// Create client with very short idle timeout
	config := DefaultConfig(serverAddr)
	config.MaxConnections = 2
	config.IdleTimeout = 10 * time.Millisecond // Very short
	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()
	
	ctx := context.Background()
	payload := make([]byte, needle.NeedleLength)
	
	// Perform an operation to create a connection
	err = client.SetBytes(ctx, payload)
	if err != nil {
		t.Errorf("Initial SET failed: %v", err)
	}
	
	// Wait for the connection to become stale
	time.Sleep(50 * time.Millisecond)
	
	// Perform another operation - this should detect stale connection and create new one
	err = client.SetBytes(ctx, payload)
	if err != nil {
		t.Errorf("SET with stale connection failed: %v", err)
	}
	
	stats := client.Stats()
	t.Logf("After stale connection handling: Active=%d, Idle=%d, Total=%d", stats.Active, stats.Idle, stats.Total)
}

func TestConnectionErrors(t *testing.T) {
	// Test with invalid server address to trigger connection errors
	config := DefaultConfig("invalid-address:99999")
	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()
	
	ctx := context.Background()
	payload := make([]byte, needle.NeedleLength)
	
	// This should fail due to invalid address
	err = client.SetBytes(ctx, payload)
	if err == nil {
		t.Error("Expected error with invalid server address")
	}
	
	// Test GetBytes with invalid address too
	hash := make([]byte, needle.HashLength)
	_, err = client.GetBytes(ctx, hash)
	if err == nil {
		t.Error("Expected error with invalid server address")
	}
}

// mockLogger is a simple logger implementation for testing
type mockLogger struct{}

func (m *mockLogger) Fatal(v ...any)                           {}
func (m *mockLogger) Fatalf(format string, v ...any)           {}
func (m *mockLogger) Error(v ...any)                           {}
func (m *mockLogger) Errorf(format string, v ...any)           {}
func (m *mockLogger) Info(v ...any)                            {}
func (m *mockLogger) Infof(format string, v ...any)            {}

func TestPool_OverflowAndClosedScenarios(t *testing.T) {
	serverAddr := startTestServer(t)
	
	t.Run("PoolFullScenario", func(t *testing.T) {
		// Create a client with pool size of 1 to easily trigger overflow
		config := DefaultConfig(serverAddr)
		config.MaxConnections = 1
		client, err := New(config)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer func() {
			if err := client.Close(); err != nil {
				t.Errorf("Failed to close client: %v", err)
			}
		}()
		
		ctx := context.Background()
		payload := make([]byte, needle.NeedleLength)
		
		// Create multiple operations to fill and overflow the pool
		for i := 0; i < 3; i++ {
			err := client.SetBytes(ctx, payload)
			if err != nil {
				t.Errorf("SET operation %d failed: %v", i, err)
			}
		}
	})
	
	t.Run("DoubleCloseScenario", func(t *testing.T) {
		config := DefaultConfig(serverAddr)
		client, err := New(config)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		
		// Close once
		err = client.Close()
		if err != nil {
			t.Errorf("First close failed: %v", err)
		}
		
		// Close again to test the already closed path
		err = client.Close()
		if err != nil {
			t.Errorf("Second close failed: %v", err)
		}
	})
}