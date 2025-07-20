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
	"github.com/nomasters/haystack/storage/memory"
)

// Execute is the main entry point for the CLI
func Execute() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "server":
		runServer(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

// printUsage prints the main usage information
func printUsage() {
	fmt.Printf(`Haystack - An ephemeral key value store

USAGE:
    haystack <command> [options]

COMMANDS:
    server      Run haystack in server mode
    help        Show this help message

Use "haystack <command> --help" for more information about a command.
`)
}

// runServer handles the server subcommand
func runServer(args []string) {
	fs := flag.NewFlagSet("server", flag.ExitOnError)

	// Define flags
	port := fs.String("port", "1337", "Port for the server listener")
	fs.StringVar(port, "p", "1337", "Port for the server listener (shorthand)")
	host := fs.String("host", "", "Hostname of server listener")
	quiet := fs.Bool("quiet", false, "Disable logging output")
	fs.BoolVar(quiet, "q", false, "Disable logging output (shorthand)")
	help := fs.Bool("help", false, "Show server command help")
	fs.BoolVar(help, "h", false, "Show server command help (shorthand)")

	// Custom usage function
	fs.Usage = func() {
		fmt.Printf(`Run haystack in server mode

USAGE:
    haystack server [options]

OPTIONS:
    -p, --port <port>     Port for the server listener (default: 1337)
        --host <host>     Hostname of server listener (default: "")
    -q, --quiet          Disable logging output
    -h, --help           Show this help message

DESCRIPTION:
    Server mode is used to run long-lived haystack servers.
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

	// Create storage backend (memory storage with 24h TTL, 2M max items)
	ctx := context.Background()
	storage := memory.New(ctx, 24*time.Hour, 2000000)

	// Create UDP server
	srv := server.New(&server.Config{
		Storage: storage,
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
	if err := storage.Close(); err != nil {
		log.Errorf("Error closing storage: %v", err)
	}

	if !*quiet {
		fmt.Println("Server stopped")
	}
}
