package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/nomasters/haystack/logger"
	"github.com/nomasters/haystack/server"
	"github.com/nomasters/haystack/storage"
	"github.com/nomasters/haystack/storage/memory"
	"github.com/nomasters/haystack/storage/mmap"
)

// runServe handles the serve subcommand
func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)

	// Define flags
	port := fs.String("port", "1337", "Port for the server listener")
	fs.StringVar(port, "p", "1337", "Port for the server listener (shorthand)")
	host := fs.String("host", "", "Hostname of server listener")
	storageType := fs.String("storage", "memory", "Storage backend: memory or mmap")
	dataDir := fs.String("data-dir", "./data", "Data directory for mmap storage")
	quiet := fs.Bool("quiet", false, "Disable logging output")
	fs.BoolVar(quiet, "q", false, "Disable logging output (shorthand)")
	help := fs.Bool("help", false, "Show server command help")
	fs.BoolVar(help, "h", false, "Show server command help (shorthand)")

	// Custom usage function
	fs.Usage = func() {
		fmt.Printf(`Run haystack in server mode

USAGE:
    haystack serve [options]

OPTIONS:
    -p, --port <port>       Port for the server listener (default: 1337)
        --host <host>       Hostname of server listener (default: "")
        --storage <type>    Storage backend: memory or mmap (default: memory)
        --data-dir <path>   Data directory for mmap storage (default: ./data)
    -q, --quiet            Disable logging output
    -h, --help             Show this help message

DESCRIPTION:
    Server mode is used to run long-lived haystack servers.
    Memory storage keeps data in RAM only.
    MMAP storage persists data to disk using memory-mapped files.
`)
	}

	// Parse flags
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Show help if requested
	if *help {
		fs.Usage()
		return
	}

	// Build address
	addr := *host + ":" + *port

	// Set up logger based on quiet flag
	var log logger.Logger
	if *quiet {
		log = logger.NewNoOp()
	} else {
		log = logger.New()
		fmt.Printf("listening on: %s\n", addr)
	}

	// Create storage backend based on type
	ctx := context.Background()
	var stor storage.GetSetCloser
	var err error

	switch *storageType {
	case "mmap":
		stor, err = mmap.New(ctx, &mmap.Config{
			DataDirectory:    *dataDir,
			TTL:              24 * time.Hour,
			MaxItems:         2000000,
			CompactThreshold: 0.25,
			GrowthChunkSize:  1024 * 1024, // 1MB
			SyncWrites:       false,
			CleanupInterval:  2 * time.Hour,
			Logger:           log,
		})
		if err != nil {
			log.Fatalf("Failed to create mmap storage: %v", err)
		}
		if !*quiet {
			fmt.Printf("using mmap storage at: %s\n", *dataDir)
		}
	case "memory":
		stor = memory.New(ctx, 24*time.Hour, 2000000)
		if !*quiet {
			fmt.Println("using in-memory storage")
		}
	default:
		fmt.Fprintf(os.Stderr, "Invalid storage type: %s\n", *storageType)
		os.Exit(1)
	}

	// Create UDP server
	srv := server.New(&server.Config{
		Storage: stor,
		Logger:  log,
	})

	// Handle graceful shutdown
	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	if !*quiet {
		fmt.Println("\nShutting down server...")
	}

	// Shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Errorf("Error during shutdown: %v", err)
	}

	// Close storage
	if err := stor.Close(); err != nil {
		log.Errorf("Error closing storage: %v", err)
	}

	if !*quiet {
		fmt.Println("Server stopped")
	}
}