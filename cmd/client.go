package cmd

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/nomasters/haystack/client"
	"github.com/nomasters/haystack/needle"
)

// runClient handles the client subcommand
func runClient(args []string) {
	if len(args) < 1 {
		printClientUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "set":
		runClientSet(args[1:])
	case "get":
		runClientGet(args[1:])
	case "stats":
		runClientStats(args[1:])
	case "help", "-h", "--help":
		printClientUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown client command: %s\n\n", args[0])
		printClientUsage()
		os.Exit(1)
	}
}

func printClientUsage() {
	fmt.Printf(`Interact with a haystack server

USAGE:
    haystack client <subcommand> [options]

SUBCOMMANDS:
    set <data>    Store data and return hash
    get <hash>    Retrieve data by hash
    stats         Show connection pool statistics
    help          Show this help message

GLOBAL OPTIONS:
    --endpoint <addr>    Server endpoint (default: localhost:1337)
                        Can also be set via HAYSTACK_ENDPOINT env var

Use "haystack client <subcommand> --help" for more information about a subcommand.
`)
}

func runClientSet(args []string) {
	fs := flag.NewFlagSet("client set", flag.ExitOnError)
	endpoint := fs.String("endpoint", getEndpoint(), "Server endpoint")
	format := fs.String("format", "text", "Output format: text, hex, or json")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help (shorthand)")

	fs.Usage = func() {
		fmt.Printf(`Store data in haystack server

USAGE:
    haystack client set [options] <data>

OPTIONS:
    --endpoint <addr>    Server endpoint (default: %s)
    --format <fmt>       Output format: text, hex, or json (default: text)
    -h, --help          Show this help message

EXAMPLES:
    haystack client set "hello world"
    haystack client set --format hex "test data"
    echo "data from stdin" | haystack client set -
`, getEndpoint())
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *help {
		fs.Usage()
		return
	}

	// Get data to store
	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Error: missing data argument\n\n")
		fs.Usage()
		os.Exit(1)
	}

	data := fs.Arg(0)
	var payload []byte

	// Handle stdin input
	if data == "-" {
		payloadBytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
			os.Exit(1)
		}
		payload = payloadBytes
	} else {
		payload = []byte(data)
	}

	// Pad or truncate payload to exactly 160 bytes
	var needlePayload [needle.PayloadLength]byte
	copy(needlePayload[:], payload)

	// Create needle from payload
	n, err := needle.New(needlePayload[:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating needle: %v\n", err)
		os.Exit(1)
	}

	// Create client
	c, err := client.New(client.DefaultConfig(*endpoint))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating client: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	// Set needle with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Set(ctx, n); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting needle: %v\n", err)
		os.Exit(1)
	}

	// Output hash based on format
	hash := n.Hash()
	hashHex := hex.EncodeToString(hash[:])
	switch *format {
	case "hex":
		fmt.Println(hashHex)
	case "json":
		result := map[string]string{
			"hash": hashHex,
			"size": fmt.Sprintf("%d", len(payload)),
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Stored with hash: %s\n", hashHex)
	}
}

func runClientGet(args []string) {
	fs := flag.NewFlagSet("client get", flag.ExitOnError)
	endpoint := fs.String("endpoint", getEndpoint(), "Server endpoint")
	format := fs.String("format", "text", "Output format: text, hex, or json")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help (shorthand)")

	fs.Usage = func() {
		fmt.Printf(`Retrieve data from haystack server

USAGE:
    haystack client get [options] <hash>

OPTIONS:
    --endpoint <addr>    Server endpoint (default: %s)
    --format <fmt>       Output format: text, hex, or json (default: text)
    -h, --help          Show this help message

EXAMPLES:
    haystack client get abc123...
    haystack client get --format hex abc123...
`, getEndpoint())
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *help {
		fs.Usage()
		return
	}

	// Get hash argument
	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Error: missing hash argument\n\n")
		fs.Usage()
		os.Exit(1)
	}

	hashStr := fs.Arg(0)

	// Decode hash from hex
	hashBytes, err := hex.DecodeString(hashStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding hash: %v\n", err)
		os.Exit(1)
	}

	if len(hashBytes) != needle.HashLength {
		fmt.Fprintf(os.Stderr, "Error: hash must be %d bytes (got %d)\n", needle.HashLength, len(hashBytes))
		os.Exit(1)
	}

	var hash needle.Hash
	copy(hash[:], hashBytes)

	// Create client
	c, err := client.New(client.DefaultConfig(*endpoint))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating client: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	// Get needle with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	n, err := c.Get(ctx, hash)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting needle: %v\n", err)
		os.Exit(1)
	}

	// Validate the hash matches
	retrievedHash := n.Hash()
	if retrievedHash != hash {
		fmt.Fprintf(os.Stderr, "Error: hash mismatch (expected %x, got %x)\n", hash, retrievedHash)
		os.Exit(1)
	}

	// Output payload based on format
	payloadArray := n.Payload()
	// Trim null bytes from the end of payload
	payload := []byte(strings.TrimRight(string(payloadArray[:]), "\x00"))

	switch *format {
	case "hex":
		fmt.Println(hex.EncodeToString(payload))
	case "json":
		result := map[string]interface{}{
			"hash":    hashStr,
			"payload": string(payload),
			"size":    len(payload),
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Print(string(payload))
		if !strings.HasSuffix(string(payload), "\n") {
			fmt.Println()
		}
	}
}

func runClientStats(args []string) {
	fs := flag.NewFlagSet("client stats", flag.ExitOnError)
	endpoint := fs.String("endpoint", getEndpoint(), "Server endpoint")
	format := fs.String("format", "text", "Output format: text or json")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help (shorthand)")

	fs.Usage = func() {
		fmt.Printf(`Show connection pool statistics

USAGE:
    haystack client stats [options]

OPTIONS:
    --endpoint <addr>    Server endpoint (default: %s)
    --format <fmt>       Output format: text or json (default: text)
    -h, --help          Show this help message
`, getEndpoint())
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *help {
		fs.Usage()
		return
	}

	// Create client
	c, err := client.New(client.DefaultConfig(*endpoint))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating client: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	// Get stats
	stats := c.Stats()

	// Output stats based on format
	switch *format {
	case "json":
		if err := json.NewEncoder(os.Stdout).Encode(stats); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Connection Pool Statistics:\n")
		fmt.Printf("  Active connections: %d\n", stats.Active)
		fmt.Printf("  Idle connections:   %d\n", stats.Idle)
		fmt.Printf("  Total created:      %d\n", stats.Total)
	}
}

// getEndpoint returns the server endpoint from environment or default
func getEndpoint() string {
	if endpoint := os.Getenv("HAYSTACK_ENDPOINT"); endpoint != "" {
		return endpoint
	}
	return "localhost:1337"
}