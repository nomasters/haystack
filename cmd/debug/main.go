package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/nomasters/haystack/client"
	"github.com/nomasters/haystack/needle"
)

func main() {
	endpoint := flag.String("endpoint", "localhost:1337", "Haystack server endpoint")
	flag.Parse()

	fmt.Printf("🔍 Haystack Debug Test\n")
	fmt.Printf("======================\n")
	fmt.Printf("Endpoint: %s\n\n", *endpoint)

	// Create client
	cfg := &client.Config{
		Address:        *endpoint,
		MaxConnections: 1,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
	}

	c, err := client.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// Test 1: Simple SET and immediate GET
	fmt.Println("Test 1: SET and immediate GET")
	fmt.Println("------------------------------")

	// Create a test needle with known data
	testData := []byte("Hello Haystack! This is a test message.")
	// Pad to 160 bytes
	paddedData := make([]byte, 160)
	copy(paddedData, testData)

	testNeedle, err := needle.New(paddedData)
	if err != nil {
		log.Fatalf("Failed to create needle: %v", err)
	}

	hash := testNeedle.Hash()
	fmt.Printf("Created needle with hash: %x\n", hash)
	fmt.Printf("Needle data (first 40 bytes): %s\n", paddedData[:40])

	// SET the needle
	fmt.Print("\nSetting needle... ")
	start := time.Now()
	err = c.Set(ctx, testNeedle)
	setDuration := time.Since(start)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("✅ (took %v)\n", setDuration)
	}

	// Small delay to ensure it's processed
	fmt.Println("Waiting 100ms for server to process...")
	time.Sleep(100 * time.Millisecond)

	// GET the needle back
	fmt.Print("Getting needle... ")
	start = time.Now()
	gotNeedle, err := c.Get(ctx, hash)
	getDuration := time.Since(start)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("✅ (took %v)\n", getDuration)

		// Verify the data matches
		if gotNeedle != nil {
			gotPayload := gotNeedle.Payload()
			matches := true
			for i := range paddedData {
				if paddedData[i] != gotPayload[i] {
					matches = false
					break
				}
			}
			if matches {
				fmt.Println("✅ Data matches!")
			} else {
				fmt.Println("❌ Data mismatch!")
				fmt.Printf("Expected: %x\n", paddedData[:40])
				fmt.Printf("Got:      %x\n", gotPayload[:40])
			}
		}
	}

	// Test 2: GET non-existent needle
	fmt.Println("\n\nTest 2: GET non-existent needle")
	fmt.Println("--------------------------------")

	randomData := make([]byte, 160)
	rand.Read(randomData)
	randomNeedle, _ := needle.New(randomData)
	randomHash := randomNeedle.Hash()

	fmt.Printf("Trying to get random hash: %x\n", randomHash)
	fmt.Print("Getting non-existent needle... ")
	start = time.Now()
	_, err = c.Get(ctx, randomHash)
	getDuration = time.Since(start)
	if err != nil {
		fmt.Printf("❌ Error (expected): %v (took %v)\n", err, getDuration)
	} else {
		fmt.Printf("⚠️  Unexpectedly found data! (took %v)\n", getDuration)
	}

	// Test 3: Rapid SET/GET cycle
	fmt.Println("\n\nTest 3: Rapid SET/GET cycle")
	fmt.Println("---------------------------")

	successCount := 0
	errorCount := 0

	for i := 0; i < 10; i++ {
		// Create unique data for each iteration
		iterData := make([]byte, 160)
		copy(iterData, fmt.Sprintf("Test iteration %d", i))

		iterNeedle, _ := needle.New(iterData)
		iterHash := iterNeedle.Hash()

		// SET
		if err := c.Set(ctx, iterNeedle); err != nil {
			fmt.Printf("  SET %d: ❌ %v\n", i, err)
			errorCount++
			continue
		}

		// Small delay
		time.Sleep(50 * time.Millisecond)

		// GET
		if _, err := c.Get(ctx, iterHash); err != nil {
			fmt.Printf("  Iteration %d: ❌ GET failed: %v\n", i, err)
			errorCount++
		} else {
			fmt.Printf("  Iteration %d: ✅\n", i)
			successCount++
		}
	}

	fmt.Printf("\nResults: %d successful, %d failed\n", successCount, errorCount)

	// Test 4: Check if original needle still exists
	fmt.Println("\n\nTest 4: Check if first needle still exists")
	fmt.Println("------------------------------------------")

	fmt.Printf("Getting original hash: %x\n", hash)
	fmt.Print("Getting original needle... ")
	start = time.Now()
	_, err = c.Get(ctx, hash)
	getDuration = time.Since(start)
	if err != nil {
		fmt.Printf("❌ Error: %v (took %v)\n", err, getDuration)
		fmt.Println("Original data may have expired or been evicted")
	} else {
		fmt.Printf("✅ Still exists! (took %v)\n", getDuration)
	}
}
