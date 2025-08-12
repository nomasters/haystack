package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nomasters/haystack/client"
	"github.com/nomasters/haystack/needle"
)

func main() {
	var (
		endpoint = flag.String("endpoint", "haystack-example-trunk.fly.dev:1337", "Haystack server endpoint")
		messages = flag.Int("messages", 100, "Number of messages to test")
		writers  = flag.Int("writers", 20, "Number of concurrent writers")
		readers  = flag.Int("readers", 20, "Number of concurrent readers")
		getOps   = flag.Int("get-ops", 0, "Total number of GET operations (0 = same as messages)")
	)
	flag.Parse()

	// Determine total GET operations
	totalGetOps := *getOps
	if totalGetOps == 0 {
		totalGetOps = *messages
	}

	fmt.Printf("🧪 Haystack Simple Test\n")
	fmt.Printf("=======================\n")
	fmt.Printf("Endpoint: %s\n", *endpoint)
	fmt.Printf("Messages: %d\n", *messages)
	fmt.Printf("Writers:  %d\n", *writers)
	fmt.Printf("Readers:  %d\n", *readers)
	fmt.Printf("GET Ops:  %d\n", totalGetOps)
	fmt.Printf("\n")

	// Create client with reasonable pool size
	cfg := &client.Config{
		Address:        *endpoint,
		MaxConnections: 300,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
	}

	c, err := client.New(cfg)
	if err != nil {
		panic(fmt.Sprintf("Failed to create client: %v", err))
	}
	defer c.Close()

	ctx := context.Background()

	// Generate test needles
	fmt.Printf("Generating %d test needles... ", *messages)
	needles := make([]*needle.Needle, *messages)
	for i := 0; i < *messages; i++ {
		data := make([]byte, 160)
		// Put message number in first bytes for verification
		data[0] = byte(i >> 8)
		data[1] = byte(i)
		// Fill rest with random data
		rand.Read(data[2:])

		n, err := needle.New(data)
		if err != nil {
			panic(fmt.Sprintf("Failed to create needle: %v", err))
		}
		needles[i] = n
	}
	fmt.Println("✅")

	// Phase 1: Write all needles
	fmt.Printf("\n📝 Phase 1: Writing %d needles with %d writers\n", *messages, *writers)
	fmt.Println("----------------------------------------")

	var writeWg sync.WaitGroup
	var writeSuccess atomic.Int32
	var writeErrors atomic.Int32

	messagesPerWriter := *messages / *writers
	extraMessages := *messages % *writers

	startWrite := time.Now()

	for w := 0; w < *writers; w++ {
		writeWg.Add(1)
		start := w * messagesPerWriter
		end := start + messagesPerWriter

		// Last writer handles extra messages
		if w == *writers-1 {
			end += extraMessages
		}

		go func(workerID int, startIdx, endIdx int) {
			defer writeWg.Done()

			for i := startIdx; i < endIdx; i++ {
				if err := c.Set(ctx, needles[i]); err != nil {
					writeErrors.Add(1)
					fmt.Printf("  Writer %d: Failed needle %d: %v\n", workerID, i, err)
				} else {
					writeSuccess.Add(1)
				}
			}
		}(w, start, end)
	}

	writeWg.Wait()
	writeDuration := time.Since(startWrite)

	fmt.Printf("\nWrite Results:\n")
	fmt.Printf("  ✅ Success: %d\n", writeSuccess.Load())
	fmt.Printf("  ❌ Errors:  %d\n", writeErrors.Load())
	fmt.Printf("  ⏱️  Duration: %v\n", writeDuration)
	fmt.Printf("  📊 Throughput: %.2f ops/sec\n", float64(*messages)/writeDuration.Seconds())

	// Wait a bit for data to settle
	fmt.Println("\nWaiting 2 second for data to settle...")
	time.Sleep(2 * time.Second)

	// Phase 2: Read needles (sustained test)
	fmt.Printf("\n📖 Phase 2: Reading %d operations across %d needles with %d readers\n", totalGetOps, *messages, *readers)
	fmt.Println("----------------------------------------")

	var readWg sync.WaitGroup
	var readSuccess atomic.Int32
	var readErrors atomic.Int32
	var dataMatches atomic.Int32

	startRead := time.Now()

	// Distribute total GET operations across readers
	opsPerReader := totalGetOps / *readers
	extraOps := totalGetOps % *readers

	for r := 0; r < *readers; r++ {
		readWg.Add(1)
		readerOps := opsPerReader
		if r == *readers-1 {
			readerOps += extraOps
		}

		go func(readerID int, numOps int) {
			defer readWg.Done()

			for op := 0; op < numOps; op++ {
				// Round-robin through available needles
				needleIdx := op % *messages
				hash := needles[needleIdx].Hash()
				gotNeedle, err := c.Get(ctx, hash)

				if err != nil {
					readErrors.Add(1)
					// Only log first few errors
					if readErrors.Load() <= 5 {
						fmt.Printf("  Reader %d: Failed op %d, needle %d (hash %x): %v\n", readerID, op, needleIdx, hash[:8], err)
					}
				} else {
					readSuccess.Add(1)

					// Verify data matches
					originalPayload := needles[needleIdx].Payload()
					gotPayload := gotNeedle.Payload()

					match := true
					for j := 0; j < len(originalPayload); j++ {
						if originalPayload[j] != gotPayload[j] {
							match = false
							break
						}
					}

					if match {
						dataMatches.Add(1)
					} else {
						fmt.Printf("  Reader %d: Data mismatch for needle %d!\n", readerID, needleIdx)
					}
				}
			}
		}(r, readerOps)
	}

	readWg.Wait()
	readDuration := time.Since(startRead)

	fmt.Printf("\nRead Results:\n")
	fmt.Printf("  ✅ Success: %d/%d\n", readSuccess.Load(), totalGetOps)
	fmt.Printf("  ✅ Data matches: %d\n", dataMatches.Load())
	fmt.Printf("  ❌ Errors:  %d\n", readErrors.Load())
	fmt.Printf("  ⏱️  Duration: %v\n", readDuration)
	fmt.Printf("  📊 Throughput: %.2f ops/sec\n", float64(totalGetOps)/readDuration.Seconds())

	// Summary
	fmt.Printf("\n========================================\n")
	fmt.Printf("📊 SUMMARY\n")
	fmt.Printf("========================================\n")
	fmt.Printf("Total write success rate: %.1f%%\n", 100.0*float64(writeSuccess.Load())/float64(*messages))
	fmt.Printf("Total read success rate:  %.1f%%\n", 100.0*float64(readSuccess.Load())/float64(totalGetOps))
	fmt.Printf("Data integrity rate:      %.1f%%\n", 100.0*float64(dataMatches.Load())/float64(totalGetOps))
}
