package client

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/nomasters/haystack/needle"
)

// Client provides a high-performance, connection-pooled interface to Haystack servers.
// It handles connection management, timeouts, and error recovery automatically.
type Client struct {
	address     string
	connPool    *connectionPool
	readTimeout time.Duration
	writeTimeout time.Duration
}

// Config holds configuration options for the Haystack client.
type Config struct {
	// Address of the Haystack server (e.g., "localhost:1337")
	Address string
	
	// Maximum number of connections in the pool (default: 10)
	MaxConnections int
	
	// Read timeout for GET operations (default: 5s)
	ReadTimeout time.Duration
	
	// Write timeout for SET operations (default: 5s)  
	WriteTimeout time.Duration
	
	// How long to keep idle connections (default: 30s)
	IdleTimeout time.Duration
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig(address string) *Config {
	return &Config{
		Address:        address,
		MaxConnections: 10,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		IdleTimeout:    30 * time.Second,
	}
}

// New creates a new Haystack client with the given configuration.
func New(config *Config) (*Client, error) {
	if config.Address == "" {
		return nil, fmt.Errorf("address is required")
	}
	
	// Apply defaults
	if config.MaxConnections == 0 {
		config.MaxConnections = 10
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 5 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 5 * time.Second
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 30 * time.Second
	}
	
	pool := newConnectionPool(config.Address, config.MaxConnections, config.IdleTimeout)
	
	return &Client{
		address:      config.Address,
		connPool:     pool,
		readTimeout:  config.ReadTimeout,
		writeTimeout: config.WriteTimeout,
	}, nil
}

// Set stores a needle on the Haystack server.
// This is a fire-and-forget operation - no response is expected.
func (c *Client) Set(ctx context.Context, n *needle.Needle) error {
	return c.SetBytes(ctx, n.Bytes())
}

// SetBytes stores raw needle bytes on the Haystack server.
// The bytes must be exactly 192 bytes (a valid needle).
func (c *Client) SetBytes(ctx context.Context, data []byte) error {
	if len(data) != needle.NeedleLength {
		return fmt.Errorf("invalid data length: expected %d bytes, got %d", needle.NeedleLength, len(data))
	}
	
	conn, err := c.connPool.Get()
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	defer c.connPool.Put(conn)
	
	// Set write timeout
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetWriteDeadline(deadline)
	} else {
		conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}
	
	_, err = conn.Write(data)
	if err != nil {
		// Mark connection as bad and don't return it to pool
		c.connPool.MarkBad(conn)
		return fmt.Errorf("failed to write needle: %w", err)
	}
	
	return nil
}

// Get retrieves a needle from the Haystack server by its hash.
func (c *Client) Get(ctx context.Context, hash needle.Hash) (*needle.Needle, error) {
	data, err := c.GetBytes(ctx, hash[:])
	if err != nil {
		return nil, err
	}
	
	return needle.FromBytes(data)
}

// GetBytes retrieves needle bytes from the Haystack server by hash.
func (c *Client) GetBytes(ctx context.Context, hashBytes []byte) ([]byte, error) {
	if len(hashBytes) != needle.HashLength {
		return nil, fmt.Errorf("invalid hash length: expected %d bytes, got %d", needle.HashLength, len(hashBytes))
	}
	
	conn, err := c.connPool.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer c.connPool.Put(conn)
	
	// Set timeouts
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Now().Add(c.readTimeout))
	}
	
	// Send hash
	_, err = conn.Write(hashBytes)
	if err != nil {
		c.connPool.MarkBad(conn)
		return nil, fmt.Errorf("failed to write hash: %w", err)
	}
	
	// Read response
	response := make([]byte, needle.NeedleLength)
	n, err := conn.Read(response)
	if err != nil {
		c.connPool.MarkBad(conn)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	if n != needle.NeedleLength {
		return nil, fmt.Errorf("invalid response length: expected %d bytes, got %d", needle.NeedleLength, n)
	}
	
	return response, nil
}

// Close closes the client and all its connections.
func (c *Client) Close() error {
	return c.connPool.Close()
}

// Stats returns connection pool statistics.
func (c *Client) Stats() PoolStats {
	return c.connPool.Stats()
}

// connectionPool manages a pool of UDP connections for reuse.
type connectionPool struct {
	address     string
	maxConns    int
	idleTimeout time.Duration
	conns       chan *pooledConn
	mu          sync.Mutex
	closed      bool
	stats       PoolStats
}

// pooledConn wraps a net.Conn with metadata for pool management.
type pooledConn struct {
	net.Conn
	created time.Time
	lastUsed time.Time
	bad     bool
}

// PoolStats provides statistics about the connection pool.
type PoolStats struct {
	Active int // Active connections
	Idle   int // Idle connections  
	Total  int // Total connections created
}

// newConnectionPool creates a new connection pool.
func newConnectionPool(address string, maxConns int, idleTimeout time.Duration) *connectionPool {
	p := &connectionPool{
		address:     address,
		maxConns:    maxConns,
		idleTimeout: idleTimeout,
		conns:       make(chan *pooledConn, maxConns),
	}
	
	// Start cleanup goroutine
	go p.cleanup()
	
	return p
}

// Get gets a connection from the pool or creates a new one.
func (p *connectionPool) Get() (net.Conn, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, fmt.Errorf("connection pool is closed")
	}
	p.stats.Active++
	p.mu.Unlock()
	
	// Try to get from pool first
	select {
	case conn := <-p.conns:
		if conn.bad || time.Since(conn.lastUsed) > p.idleTimeout {
			conn.Close()
			return p.createConn()
		}
		conn.lastUsed = time.Now()
		return conn, nil
	default:
		// Pool empty, create new connection
		return p.createConn()
	}
}

// Put returns a connection to the pool.
func (p *connectionPool) Put(conn net.Conn) {
	p.mu.Lock()
	p.stats.Active--
	p.mu.Unlock()
	
	if pooled, ok := conn.(*pooledConn); ok && !pooled.bad {
		pooled.lastUsed = time.Now()
		select {
		case p.conns <- pooled:
			// Successfully returned to pool
		default:
			// Pool full, close connection
			conn.Close()
		}
	} else {
		// Bad connection or not pooled, close it
		conn.Close()
	}
}

// MarkBad marks a connection as bad so it won't be reused.
func (p *connectionPool) MarkBad(conn net.Conn) {
	if pooled, ok := conn.(*pooledConn); ok {
		pooled.bad = true
	}
}

// createConn creates a new connection.
func (p *connectionPool) createConn() (*pooledConn, error) {
	conn, err := net.Dial("udp", p.address)
	if err != nil {
		return nil, err
	}
	
	p.mu.Lock()
	p.stats.Total++
	p.mu.Unlock()
	
	return &pooledConn{
		Conn:     conn,
		created:  time.Now(),
		lastUsed: time.Now(),
	}, nil
}

// cleanup removes idle connections from the pool.
func (p *connectionPool) cleanup() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return
		}
		p.mu.Unlock()
		
		// Clean up idle connections
		select {
		case conn := <-p.conns:
			if time.Since(conn.lastUsed) > p.idleTimeout {
				conn.Close()
			} else {
				// Put it back
				select {
				case p.conns <- conn:
				default:
					conn.Close()
				}
			}
		default:
			// No connections to clean
		}
	}
}

// Close closes all connections in the pool.
func (p *connectionPool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()
	
	close(p.conns)
	
	// Close all pooled connections
	for conn := range p.conns {
		conn.Close()
	}
	
	return nil
}

// Stats returns current pool statistics.
func (p *connectionPool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	stats := p.stats
	stats.Idle = len(p.conns)
	return stats
}