# haystack

A tiny, ephemeral, quiet, content-addressed key/value store.

## Overview

Haystack is a minimalist key/value store built for simplicity and efficiency. It operates over UDP with fixed-size messages, uses content-addressing via SHA256 hashes, and automatically expires data after a configurable time window.

## Key Features

- **Content-Addressed**: Keys are SHA256 hashes of the content itself - no separate key management needed
- **Fixed-Size Messages**: All network messages are exactly 192 bytes (writes) or 32 bytes (reads)
- **Ephemeral Storage**: Data automatically expires after a configurable TTL (default 24 hours)
- **UDP-Only**: Lightweight, stateless protocol with minimal overhead
- **No Authentication**: Content validity is proven by SHA256 hash matching
- **Zero Server Confirmation**: Write operations receive no response by design

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/nomasters/haystack.git
cd haystack

# Build and install
make install
```

### Using Go

```bash
go install github.com/nomasters/haystack@latest
```

## Quick Start

### Start a Server

```bash
# Default configuration (localhost:1337)
haystack serve

# Custom host and port
haystack serve -H 0.0.0.0 -p 9000
```

### Client Operations

```bash
# Set a value (returns the SHA256 hash)
haystack client set "hello world"

# Get a value using its hash
haystack client get <hash>

# Use pipes for content
echo "my message" | haystack client set -
```

### Go Client Library

```go
package main

import (
    "context"
    "fmt"
    "github.com/nomasters/haystack/client"
)

func main() {
    // Create a client
    c, err := client.New("localhost:1337")
    if err != nil {
        panic(err)
    }
    defer c.Close()

    // Store data
    hash, err := c.Set(context.Background(), []byte("hello world"))
    if err != nil {
        panic(err)
    }

    // Retrieve data
    data, err := c.Get(context.Background(), hash)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Retrieved: %s\n", data)
}
```

## The Needle Protocol

A message in Haystack is called a "Needle". Each needle consists of a 32-byte SHA256 hash followed by a 160-byte payload, totaling exactly 192 bytes.

```text
| hash           | payload                                                                        |
|----------------|--------------------------------------------------------------------------------|
| 32 bytes       | 160 bytes                                                                      |
```

This fixed size enables:

- Single UDP packet transmission
- Consistent network behavior
- Message chaining for larger payloads

### Message Chaining Example

The 160-byte payload is large enough to contain encrypted data and a reference to the next chunk:

```text
| needle                                                                                           |
|--------------------------------------------------------------------------------------------------|
| hash           | payload                                                                         |
|----------------|---------------------------------------------------------------------------------|
|                | nonce      | encrypted payload                                                  |
|                |------------|--------------------------------------------------------------------|  
|                |            | next key       | padded message                                    |
|                |            |----------------|---------------------------------------------------|
| 32 bytes       | 24 bytes   | 32 bytes       | 104 bytes                                         |
```

## Protocol Operations

### Read Request (32 bytes)

Send a 32-byte SHA256 hash to retrieve the associated needle. If found, the server responds with the complete 192-byte needle.

### Write Request (192 bytes)

Send a complete 192-byte needle. The server validates that the hash matches the payload and stores it if valid. No response is sent (by design).

## Architecture

Haystack follows a modular design with clear separation of concerns:

- **Needle Package**: Core message structure and validation
- **Storage Package**: Interface-based storage abstraction with TTL support
- **Server Package**: High-performance UDP server with zero-copy buffer pools
- **Client Package**: Production-ready Go client with connection pooling and retry logic

## Deployment

### Docker Images

Haystack uses a hermetic build system with content-based tagging. Images are built once and promoted through their lifecycle. Supports 64-bit platforms only (amd64, arm64).

#### Pull and Run

```bash
# Run the latest build from main branch
docker run -p 1337:1337/udp nomasters/haystack:main serve

# Run a specific version (immutable tags)
docker run -p 1337:1337/udp nomasters/haystack:v0.1.0 serve

# Run with custom configuration
docker run -p 1337:1337/udp nomasters/haystack:v0.1.0 serve -H 0.0.0.0 -p 1337
```

#### Available Tags

- `sha-<commit>`: Specific commit SHA
- `tree-<hash>`: Content-based hash (same source = same tag)
- `main`: Latest build from main branch
- `v0.1.0`: Specific release version (immutable)

### Deploy to Fly.io

Create a `fly.toml` configuration:

```toml
app = "haystack"

[build]
  image = "nomasters/haystack:v0.1.0"

[[services]]
  internal_port = 1337
  protocol = "udp"

  [[services.ports]]
    port = 1337
```

Then deploy:

```bash
fly deploy
```

## Development

### Building from Source

```bash
# Install locally
make install
```

### Docker Development

The build system uses content-based hashing to ensure hermetic, reproducible builds. By default, builds require a clean git working directory to ensure only committed code is deployed.

```bash
# Show current build info
make docker-info

# Build locally (requires clean git working directory)
make docker-build

# Build with uncommitted changes (local testing only)
SKIP_GIT_CHECK=true make docker-build

# Build and push to registry (always requires clean git)
make docker-push

# Check if image exists for current source
make docker-exists

# Promote image with version tag
make docker-promote TAGS="v0.1.0"
```

### Testing & Quality Checks

```bash
# Run tests with race detection
make test

# Generate coverage report
make coverage

# Run all quality checks (fmt, vet, test, shellcheck)
make check

# Individual checks
make fmt         # Format Go code
make lint        # Run go vet
make shellcheck  # Check shell scripts

# Run specific tests
go test -v -run TestName ./path/to/package
```

### Project Structure

```text
haystack/
├── needle/     # Core message format
├── storage/    # Storage interface and implementations
├── server/     # UDP server implementation
├── client/     # Go client library
└── cmd/        # CLI implementation
```

## Design Philosophy

Haystack embraces several core principles:

1. **Simplicity**: Minimal protocol, clear semantics, predictable behavior
2. **Performance**: Zero-copy operations, buffer pooling, efficient packet handling
3. **Reliability**: Content-addressed storage ensures data integrity
4. **Efficiency**: Fixed-size messages and stateless UDP protocol

## Use Cases

- Temporary data storage with automatic expiration
- Distributed caching
- Message passing between services
- Content-addressed storage needs
- Lightweight key/value operations

## Contributing

Contributions are welcome! Please ensure:

- All tests pass with race detection enabled
- Code follows standard Go conventions
- New features include appropriate tests
- Changes maintain backward compatibility

## License

The Unlicense - This is free and unencumbered software released into the public domain. See [LICENSE](LICENSE) for details.

