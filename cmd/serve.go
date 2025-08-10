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
	addr := fs.String("addr", getAddr(), "Server address (host:port)")
	fs.StringVar(addr, "a", getAddr(), "Server address (host:port) (shorthand)")
	storageType := fs.String("storage", getStorage(), "Storage backend: memory or mmap")
	dataDir := fs.String("data-dir", getDataDir(), "Data directory for mmap storage")
	logLevel := fs.String("log-level", getLogLevel(), "Log level: debug, info, error, or silent")
	quiet := fs.Bool("quiet", false, "Disable logging output (same as --log-level=silent)")
	fs.BoolVar(quiet, "q", false, "Disable logging output (shorthand)")
	help := fs.Bool("help", false, "Show server command help")
	fs.BoolVar(help, "h", false, "Show server command help (shorthand)")

	// Custom usage function
	fs.Usage = func() {
		fmt.Printf(`Run haystack in server mode

USAGE:
    haystack serve [options]

OPTIONS:
    -a, --addr <addr>       Server address (host:port) (default: %s)
        --storage <type>    Storage backend: memory or mmap (default: %s)
        --data-dir <path>   Data directory for mmap storage (default: %s)
        --log-level <level> Log level: debug, info, error, or silent (default: %s)
    -q, --quiet            Disable logging output (same as --log-level=silent)
    -h, --help             Show this help message

ENVIRONMENT VARIABLES:
    HAYSTACK_ADDR          Server address (overridden by --addr)
    HAYSTACK_STORAGE       Storage backend (overridden by --storage)
    HAYSTACK_DATA_DIR      Data directory (overridden by --data-dir)
    HAYSTACK_LOG_LEVEL     Log level (overridden by --log-level)

DESCRIPTION:
    Server mode is used to run long-lived haystack servers.
    Memory storage keeps data in RAM only.
    MMAP storage persists data to disk using memory-mapped files.
`, getAddr(), getStorage(), getDataDir(), getLogLevel())
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

	// Determine effective log level
	effectiveLogLevel := *logLevel
	if *quiet {
		effectiveLogLevel = "silent"
	}

	// Set up logger based on log level
	var log logger.Logger
	if effectiveLogLevel == "silent" {
		log = logger.NewNoOp()
	} else {
		log = logger.NewWithLevel(effectiveLogLevel)
		fmt.Printf("listening on: %s (log level: %s)\n", *addr, effectiveLogLevel)
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
		if err := srv.ListenAndServe(*addr); err != nil {
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

// getAddr returns the server address from environment or default
func getAddr() string {
	if addr := os.Getenv("HAYSTACK_ADDR"); addr != "" {
		return addr
	}
	return ":1337"
}

// getStorage returns the storage type from environment or default
func getStorage() string {
	if storage := os.Getenv("HAYSTACK_STORAGE"); storage != "" {
		return storage
	}
	return "memory"
}

// getDataDir returns the data directory from environment or default
func getDataDir() string {
	if dataDir := os.Getenv("HAYSTACK_DATA_DIR"); dataDir != "" {
		return dataDir
	}
	return "./data"
}

// getLogLevel returns the log level from environment or default
func getLogLevel() string {
	if level := os.Getenv("HAYSTACK_LOG_LEVEL"); level != "" {
		return level
	}
	return "info"
}
