package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/nomasters/haystack/client"
	"github.com/nomasters/haystack/needle"
)

type Stats struct {
	setOps     atomic.Int64
	getOps     atomic.Int64
	setErrors  atomic.Int64
	getErrors  atomic.Int64
	setLatency atomic.Int64 // cumulative microseconds
	getLatency atomic.Int64 // cumulative microseconds
}

func (s *Stats) RecordSet(duration time.Duration, err error) {
	s.setOps.Add(1)
	s.setLatency.Add(duration.Microseconds())
	if err != nil {
		s.setErrors.Add(1)
	}
}

func (s *Stats) RecordGet(duration time.Duration, err error) {
	s.getOps.Add(1)
	s.getLatency.Add(duration.Microseconds())
	if err != nil {
		s.getErrors.Add(1)
	}
}

func (s *Stats) Report() {
	setOps := s.setOps.Load()
	getOps := s.getOps.Load()
	setErrors := s.setErrors.Load()
	getErrors := s.getErrors.Load()
	setLatencyTotal := s.setLatency.Load()
	getLatencyTotal := s.getLatency.Load()

	fmt.Printf("\n=== Performance Report ===\n")
	fmt.Printf("Operations:\n")
	fmt.Printf("  SET: %d ops (%d errors, %.2f%% success)\n",
		setOps, setErrors,
		100.0*float64(setOps-setErrors)/float64(max(setOps, 1)))
	fmt.Printf("  GET: %d ops (%d errors, %.2f%% success)\n",
		getOps, getErrors,
		100.0*float64(getOps-getErrors)/float64(max(getOps, 1)))

	fmt.Printf("\nLatency:\n")
	if setOps > 0 {
		successfulSets := setOps - setErrors
		if successfulSets > 0 {
			// Note: This includes both successful and failed operations
			avgSetLatency := float64(setLatencyTotal) / float64(setOps)
			fmt.Printf("  SET avg: %.2f ms (includes timeouts)\n", avgSetLatency/1000)
		}
	}
	if getOps > 0 {
		successfulGets := getOps - getErrors
		if successfulGets > 0 {
			// Note: This includes both successful and failed operations
			avgGetLatency := float64(getLatencyTotal) / float64(getOps)
			fmt.Printf("  GET avg: %.2f ms (includes timeouts)\n", avgGetLatency/1000)

			// Estimate: If failures are timeouts, calculate success-only average
			if getErrors > 0 {
				timeoutMs := float64(10000) // 10 second timeout in ms
				totalTimeoutMs := float64(getErrors) * timeoutMs
				successLatencyTotal := float64(getLatencyTotal) - (totalTimeoutMs * 1000) // Convert to microseconds
				if successLatencyTotal > 0 && successfulGets > 0 {
					avgSuccessLatency := successLatencyTotal / float64(successfulGets)
					fmt.Printf("  GET successful only: ~%.2f ms\n", avgSuccessLatency/1000)
				}
			}
		}
	}
}

func (s *Stats) Reset() {
	s.setOps.Store(0)
	s.getOps.Store(0)
	s.setErrors.Store(0)
	s.getErrors.Store(0)
	s.setLatency.Store(0)
	s.getLatency.Store(0)
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// generateNeedles creates a corpus of random needles
func generateNeedles(sizeMB int) ([]*needle.Needle, error) {
	needleSize := 192 // bytes per needle
	needlesPerMB := (1024 * 1024) / needleSize
	totalNeedles := sizeMB * needlesPerMB

	fmt.Printf("Generating %d needles (~%d MB)... ", totalNeedles, sizeMB)

	needles := make([]*needle.Needle, totalNeedles)
	for i := 0; i < totalNeedles; i++ {
		data := make([]byte, 160)
		if _, err := rand.Read(data); err != nil {
			return nil, fmt.Errorf("failed to generate random data: %w", err)
		}

		n, err := needle.New(data)
		if err != nil {
			return nil, fmt.Errorf("failed to create needle: %w", err)
		}
		needles[i] = n

		// Progress indicator
		if i > 0 && i%(needlesPerMB*10) == 0 {
			fmt.Print(".")
		}
	}

	fmt.Println(" ✅")
	return needles, nil
}

// setWorker performs SET operations without blocking
func setWorker(ctx context.Context, c *client.Client, needles []*needle.Needle, stats *Stats, wg *sync.WaitGroup, workerId int, numWorkers int, maxConcurrent int) {
	defer wg.Done()

	// Create a semaphore to limit concurrent requests per worker
	sem := make(chan struct{}, maxConcurrent) // Limit in-flight requests per worker

	// Each worker handles needles with index % numWorkers == workerId
	for i := workerId; i < len(needles); i += numWorkers {
		select {
		case <-ctx.Done():
			// Drain remaining requests
			for j := 0; j < len(sem); j++ {
				select {
				case <-sem:
				default:
					return
				}
			}
			return
		default:
			// Acquire semaphore slot
			sem <- struct{}{}

			// Fire off the SET request in a goroutine (non-blocking)
			go func(needle *needle.Needle) {
				defer func() { <-sem }() // Release semaphore slot when done

				start := time.Now()
				err := c.Set(ctx, needle)
				stats.RecordSet(time.Since(start), err)
			}(needles[i])

			// Small delay every N requests to prevent CPU spinning
			if i%100 == 0 {
				time.Sleep(1 * time.Microsecond)
			}
		}
	}

	// Wait for all in-flight requests to complete
	for i := 0; i < cap(sem); i++ {
		sem <- struct{}{}
	}
}

// getWorker performs GET operations without blocking
func getWorker(ctx context.Context, c *client.Client, needles []*needle.Needle, stats *Stats, wg *sync.WaitGroup, workerId int, maxConcurrent int) {
	defer wg.Done()

	// Create a semaphore to limit concurrent requests per worker
	// This prevents overwhelming the system
	sem := make(chan struct{}, maxConcurrent) // Limit in-flight requests per worker

	// Launch requests continuously
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			// Drain remaining requests
			for j := 0; j < len(sem); j++ {
				select {
				case <-sem:
				default:
					return
				}
			}
			return
		default:
			// Acquire semaphore slot
			sem <- struct{}{}

			// Pick a needle to GET (round-robin through corpus)
			idx := (workerId*1000 + i) % len(needles)
			hash := needles[idx].Hash()

			// Fire off the request in a goroutine (non-blocking)
			go func(h needle.Hash) {
				defer func() { <-sem }() // Release semaphore slot when done

				start := time.Now()
				_, err := c.Get(ctx, h)
				stats.RecordGet(time.Since(start), err)

				// Log first few errors for debugging
				if err != nil && stats.getErrors.Load() < 5 {
					fmt.Printf("[GET Error] Hash %x: %v\n", h, err)
				}
			}(hash)

			// Small delay to prevent CPU spinning
			if i%100 == 0 {
				time.Sleep(1 * time.Microsecond)
			}
		}
	}
}

func main() {
	var (
		endpoint    = flag.String("endpoint", "localhost:1337", "Haystack server endpoint")
		sizeMB      = flag.Int("size", 100, "Size of test corpus in MB")
		setWorkers  = flag.Int("set-workers", 5, "Number of SET workers")
		getWorkers  = flag.Int("get-workers", 10, "Number of GET workers")
		getDuration = flag.Duration("get-duration", 30*time.Second, "Duration for GET test")
		poolSize    = flag.Int("pool", 0, "Connection pool size (0 = auto, based on workers)")
		reportFreq  = flag.Duration("report", 5*time.Second, "Reporting frequency")
	)
	flag.Parse()

	// Auto-size the pool based on workers if not specified
	actualPoolSize := *poolSize
	if actualPoolSize == 0 {
		// Use 2x the max number of workers as pool size
		if *setWorkers > *getWorkers {
			actualPoolSize = 2 * *setWorkers
		} else {
			actualPoolSize = 2 * *getWorkers
		}
		if actualPoolSize < 10 {
			actualPoolSize = 10
		}
		if actualPoolSize > 100 {
			actualPoolSize = 100 // Cap at 100 to be reasonable
		}
	}

	// Calculate per-worker concurrency based on total workers
	maxConcurrentPerWorker := 100
	totalWorkers := *setWorkers
	if *getWorkers > totalWorkers {
		totalWorkers = *getWorkers
	}
	if totalWorkers > 10 {
		// Scale down per-worker concurrency for many workers
		maxConcurrentPerWorker = 1000 / totalWorkers
		if maxConcurrentPerWorker < 10 {
			maxConcurrentPerWorker = 10
		}
	}

	fmt.Printf("🔥 Haystack Stress Test\n")
	fmt.Printf("========================\n")
	fmt.Printf("Endpoint:        %s\n", *endpoint)
	fmt.Printf("Corpus Size:     %d MB\n", *sizeMB)
	fmt.Printf("SET Workers:     %d\n", *setWorkers)
	fmt.Printf("GET Workers:     %d\n", *getWorkers)
	fmt.Printf("GET Duration:    %s\n", *getDuration)
	fmt.Printf("Pool Size:       %d\n", actualPoolSize)
	fmt.Printf("Concurrency:     %d per worker\n", maxConcurrentPerWorker)
	fmt.Printf("\n")

	// Create client with custom pool size
	cfg := &client.Config{
		Address:        *endpoint,
		MaxConnections: actualPoolSize,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
	}

	c, err := client.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Test connectivity
	fmt.Print("Testing connectivity... ")
	ctx := context.Background()
	testData := make([]byte, 160)
	rand.Read(testData)
	testNeedle, _ := needle.New(testData)

	if err := c.Set(ctx, testNeedle); err != nil {
		fmt.Printf("❌\n")
		log.Fatalf("Failed to connect: %v", err)
	}
	fmt.Printf("✅\n\n")

	// Generate test corpus
	needles, err := generateNeedles(*sizeMB)
	if err != nil {
		log.Fatalf("Failed to generate needles: %v", err)
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Statistics
	stats := &Stats{}

	// Phase 1: SET all needles
	fmt.Printf("\n📤 Phase 1: Storing %d needles with %d workers\n", len(needles), *setWorkers)
	fmt.Println("========================================")

	setCtx, setCancel := context.WithCancel(ctx)
	var setWg sync.WaitGroup

	// Progress tracking for SET phase
	setStartTime := time.Now()
	setReportTicker := time.NewTicker(*reportFreq)
	defer setReportTicker.Stop()

	// Start SET workers
	for i := 0; i < *setWorkers; i++ {
		setWg.Add(1)
		go setWorker(setCtx, c, needles, stats, &setWg, i, *setWorkers, maxConcurrentPerWorker)
	}

	// Monitor SET phase
	go func() {
		for {
			select {
			case <-setCtx.Done():
				return
			case <-setReportTicker.C:
				setOps := stats.setOps.Load()
				elapsed := time.Since(setStartTime)
				remaining := len(needles) - int(setOps)
				fmt.Printf("[%s] Stored: %d/%d (%.1f/s), Remaining: %d\n",
					elapsed.Round(time.Second),
					setOps, len(needles),
					float64(setOps)/elapsed.Seconds(),
					remaining)

				// Check if all needles are stored
				if int(setOps) >= len(needles) {
					setCancel()
					return
				}
			case <-sigChan:
				fmt.Println("\n⚠️  Interrupted!")
				setCancel()
				return
			}
		}
	}()

	// Wait for SET phase to complete
	setWg.Wait()
	setCancel()

	setDuration := time.Since(setStartTime)
	fmt.Printf("\n✅ SET Phase Complete!\n")
	fmt.Printf("   Stored: %d needles in %s\n", stats.setOps.Load(), setDuration)
	fmt.Printf("   Throughput: %.2f ops/sec\n", float64(stats.setOps.Load())/setDuration.Seconds())
	fmt.Printf("   Errors: %d\n", stats.setErrors.Load())

	// Small delay to ensure data is actually stored on server
	fmt.Println("\nWaiting 2 seconds for data to settle...")
	time.Sleep(2 * time.Second)

	// Reset stats for GET phase
	getStats := &Stats{}

	// Phase 2: GET operations
	fmt.Printf("\n📥 Phase 2: Reading needles with %d workers for %s\n", *getWorkers, *getDuration)
	fmt.Println("==============================================")

	getCtx, getCancel := context.WithTimeout(ctx, *getDuration)
	defer getCancel()

	var getWg sync.WaitGroup
	getStartTime := time.Now()
	getReportTicker := time.NewTicker(*reportFreq)
	defer getReportTicker.Stop()

	// Start GET workers
	for i := 0; i < *getWorkers; i++ {
		getWg.Add(1)
		go getWorker(getCtx, c, needles, getStats, &getWg, i, maxConcurrentPerWorker)
	}

	// Monitor GET phase
	for {
		select {
		case <-sigChan:
			fmt.Println("\n⚠️  Interrupted!")
			getCancel()
			getWg.Wait()
			getStats.Report()
			return

		case <-getCtx.Done():
			fmt.Println("\n✅ GET Phase Complete!")
			getWg.Wait()

			getDuration := time.Since(getStartTime)
			fmt.Printf("   Read: %d operations in %s\n", getStats.getOps.Load(), getDuration)
			fmt.Printf("   Throughput: %.2f ops/sec\n", float64(getStats.getOps.Load())/getDuration.Seconds())
			fmt.Printf("   Errors: %d\n", getStats.getErrors.Load())

			// Final combined report
			fmt.Println("\n==================================================")
			fmt.Println("📊 FINAL SUMMARY")
			fmt.Println("==================================================")

			fmt.Printf("\nSET Performance:\n")
			fmt.Printf("  Operations: %d\n", stats.setOps.Load())
			fmt.Printf("  Errors: %d\n", stats.setErrors.Load())
			fmt.Printf("  Avg Latency: %.2f ms\n", float64(stats.setLatency.Load())/float64(max(stats.setOps.Load(), 1))/1000)
			fmt.Printf("  Throughput: %.2f ops/sec\n", float64(stats.setOps.Load())/setDuration.Seconds())

			fmt.Printf("\nGET Performance:\n")
			fmt.Printf("  Operations: %d\n", getStats.getOps.Load())
			fmt.Printf("  Errors: %d\n", getStats.getErrors.Load())
			fmt.Printf("  Avg Latency: %.2f ms\n", float64(getStats.getLatency.Load())/float64(max(getStats.getOps.Load(), 1))/1000)
			fmt.Printf("  Throughput: %.2f ops/sec\n", float64(getStats.getOps.Load())/getDuration.Seconds())

			fmt.Printf("\nTotal Operations: %d\n", stats.setOps.Load()+getStats.getOps.Load())
			return

		case <-getReportTicker.C:
			getOps := getStats.getOps.Load()
			elapsed := time.Since(getStartTime)
			fmt.Printf("[%s] GET: %d ops (%.1f/s), Errors: %d\n",
				elapsed.Round(time.Second),
				getOps,
				float64(getOps)/elapsed.Seconds(),
				getStats.getErrors.Load())
		}
	}
}
