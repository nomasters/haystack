package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nomasters/haystack/client"
	"github.com/nomasters/haystack/needle"
)

func main() {
	// Create a Haystack client
	config := client.DefaultConfig("localhost:1337")
	config.MaxConnections = 10
	config.ReadTimeout = 5 * time.Second
	config.WriteTimeout = 5 * time.Second
	
	haystackClient, err := client.New(config)
	if err != nil {
		log.Fatalf("Failed to create Haystack client: %v", err)
	}
	defer haystackClient.Close()
	
	ctx := context.Background()
	
	// Example 1: Store and retrieve text data
	fmt.Println("=== Example 1: Text Data ===")
	
	message := "Hello, Haystack! This is a sample message."
	payload := make([]byte, needle.PayloadLength)
	copy(payload, []byte(message))
	
	// Create needle
	n, err := needle.New(payload)
	if err != nil {
		log.Fatalf("Failed to create needle: %v", err)
	}
	
	// Store the data
	fmt.Printf("Storing message: %s\n", message)
	fmt.Printf("Hash: %x\n", n.Hash())
	
	err = haystackClient.Set(ctx, n)
	if err != nil {
		log.Fatalf("Failed to store needle: %v", err)
	}
	
	// Retrieve the data
	hash := n.Hash()
	retrievedNeedle, err := haystackClient.Get(ctx, hash)
	if err != nil {
		log.Fatalf("Failed to retrieve needle: %v", err)
	}
	
	retrievedPayload := retrievedNeedle.Payload()
	retrievedMessage := string(retrievedPayload[:len(message)])
	
	fmt.Printf("Retrieved message: %s\n", retrievedMessage)
	fmt.Printf("Match: %t\n\n", message == retrievedMessage)
	
	// Example 2: Store multiple items and show connection reuse
	fmt.Println("=== Example 2: Multiple Operations ===")
	
	messages := []string{
		"First message",
		"Second message", 
		"Third message",
		"Fourth message",
		"Fifth message",
	}
	
	hashes := make([]needle.Hash, len(messages))
	
	// Store all messages
	for i, msg := range messages {
		payload := make([]byte, needle.PayloadLength)
		copy(payload, []byte(msg))
		
		n, err := needle.New(payload)
		if err != nil {
			log.Fatalf("Failed to create needle %d: %v", i, err)
		}
		
		err = haystackClient.Set(ctx, n)
		if err != nil {
			log.Fatalf("Failed to store needle %d: %v", i, err)
		}
		
		hashes[i] = n.Hash()
		hash := n.Hash()
		fmt.Printf("Stored message %d: %s (hash: %x)\n", i+1, msg, hash[:8])
	}
	
	// Retrieve all messages
	fmt.Println("\nRetrieving messages:")
	for i, hash := range hashes {
		retrievedNeedle, err := haystackClient.Get(ctx, hash)
		if err != nil {
			log.Fatalf("Failed to retrieve needle %d: %v", i, err)
		}
		
		retrievedPayload := retrievedNeedle.Payload()
		retrievedMessage := string(retrievedPayload[:len(messages[i])])
		
		fmt.Printf("Retrieved message %d: %s\n", i+1, retrievedMessage)
	}
	
	// Example 3: Show connection pool statistics
	fmt.Println("\n=== Example 3: Connection Pool Stats ===")
	
	stats := haystackClient.Stats()
	fmt.Printf("Active connections: %d\n", stats.Active)
	fmt.Printf("Idle connections: %d\n", stats.Idle)
	fmt.Printf("Total connections created: %d\n", stats.Total)
	
	// Example 4: Error handling
	fmt.Println("\n=== Example 4: Error Handling ===")
	
	// Try to get a non-existent needle
	var nonExistentHash needle.Hash
	for i := range nonExistentHash {
		nonExistentHash[i] = 0xFF
	}
	
	fmt.Printf("Trying to get non-existent hash: %x\n", nonExistentHash[:8])
	_, err = haystackClient.Get(ctx, nonExistentHash)
	if err != nil {
		fmt.Printf("Expected error occurred: %v\n", err)
	} else {
		fmt.Println("Unexpected: no error occurred")
	}
	
	fmt.Println("\n=== Client Demo Complete ===")
}