package cmd

import (
	"fmt"
	"os"
)

// Execute is the main entry point for the CLI
func Execute() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe(os.Args[2:])
	case "client":
		runClient(os.Args[2:])
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
    serve       Run haystack in server mode
    client      Interact with a haystack server
    help        Show this help message

Use "haystack <command> --help" for more information about a command.
`)
}

