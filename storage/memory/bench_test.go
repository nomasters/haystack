package memory

import (
	"context"
	"testing"
	"time"

	"github.com/nomasters/haystack/needle"
)

func BenchmarkMemoryStore_Set(b *testing.B) {
	ctx := context.Background()
	store := New(ctx, time.Hour, 1000000)
	defer func() {
		if err := store.Close(); err != nil {
			b.Fatalf("Failed to close store: %v", err)
		}
	}()

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
		err := store.Set(testNeedle)
		if err != nil {
			b.Fatalf("Set operation failed: %v", err)
		}
	}
}

func BenchmarkMemoryStore_Get(b *testing.B) {
	ctx := context.Background()
	store := New(ctx, time.Hour, 1000000)
	defer func() {
		if err := store.Close(); err != nil {
			b.Fatalf("Failed to close store: %v", err)
		}
	}()

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
		b.Fatalf("Failed to store test needle: %v", err)
	}

	hash := testNeedle.Hash()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := store.Get(hash)
		if err != nil {
			b.Fatalf("Get operation failed: %v", err)
		}
	}
}

func BenchmarkMemoryStore_SetGet_Mixed(b *testing.B) {
	ctx := context.Background()
	store := New(ctx, time.Hour, 1000000)
	defer func() {
		if err := store.Close(); err != nil {
			b.Fatalf("Failed to close store: %v", err)
		}
	}()

	payload := make([]byte, needle.PayloadLength)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	testNeedle, err := needle.New(payload)
	if err != nil {
		b.Fatalf("Failed to create test needle: %v", err)
	}

	hash := testNeedle.Hash()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if i%4 == 0 {
			err := store.Set(testNeedle)
			if err != nil {
				b.Fatalf("Set operation failed: %v", err)
			}
		} else {
			_, err := store.Get(hash)
			if err != nil {
				b.Fatalf("Get operation failed: %v", err)
			}
		}
	}
}
