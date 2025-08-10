package client

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/nomasters/haystack/logger"
	"github.com/nomasters/haystack/needle"
)

// Client provides a high-performance, connection-pooled interface to Haystack servers.
// It handles connection management, timeouts, and error recovery automatically.
type Client struct {
	address      string
	logger       logger.Logger
	connPool     *connectionPool
	readTimeout  time.Duration
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

	// Logger for error and debug messages (optional, uses NoOp if nil)
	Logger logger.Logger
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig(address string) *Config {
	return &Config{
		Address:        address,
		MaxConnections: 10,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		IdleTimeout:    30 * time.Second,
		Logger:         nil, // Will use NoOp by default
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

	// Use NoOp logger if none provided
	log := config.Logger
	if log == nil {
		log = logger.NewNoOp()
	}

	pool := newConnectionPool(config.Address, config.MaxConnections, config.IdleTimeout, log)

	return &Client{
		address:      config.Address,
		logger:       log,
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
		if err := conn.SetWriteDeadline(deadline); err != nil {
			c.logger.Errorf("Failed to set write deadline: %v", err)
		}
	} else {
		if err := conn.SetWriteDeadline(time.Now().Add(c.writeTimeout)); err != nil {
			c.logger.Errorf("Failed to set write timeout: %v", err)
		}
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
		if err := conn.SetDeadline(deadline); err != nil {
			c.logger.Errorf("Failed to set read deadline: %v", err)
		}
	} else {
		if err := conn.SetDeadline(time.Now().Add(c.readTimeout)); err != nil {
			c.logger.Errorf("Failed to set read timeout: %v", err)
		}
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

// udpConn wraps a PacketConn to implement net.Conn for unconnected UDP.
// This allows receiving responses from any address, which is needed for NAT traversal.
type udpConn struct {
	net.PacketConn
	serverAddr net.Addr
}

// Read implements net.Conn Read for unconnected UDP.
func (c *udpConn) Read(b []byte) (int, error) {
	// ReadFrom accepts packets from any address
	// This is crucial for NAT traversal where responses may come from different addresses
	n, _, err := c.PacketConn.ReadFrom(b)
	return n, err
}

// Write implements net.Conn Write for unconnected UDP.
func (c *udpConn) Write(b []byte) (int, error) {
	// Always send to the server address
	return c.PacketConn.WriteTo(b, c.serverAddr)
}

// RemoteAddr implements net.Conn RemoteAddr.
func (c *udpConn) RemoteAddr() net.Addr {
	return c.serverAddr
}

// connectionPool manages a pool of UDP connections for reuse.
type connectionPool struct {
	address     string
	maxConns    int
	idleTimeout time.Duration
	logger      logger.Logger
	conns       chan *pooledConn
	mu          sync.Mutex
	closed      bool
	stats       PoolStats
}

// pooledConn wraps a net.Conn with metadata for pool management.
type pooledConn struct {
	net.Conn
	created  time.Time
	lastUsed time.Time
	bad      bool
}

// PoolStats provides statistics about the connection pool.
type PoolStats struct {
	Active int // Active connections
	Idle   int // Idle connections
	Total  int // Total connections created
}

// newConnectionPool creates a new connection pool.
func newConnectionPool(address string, maxConns int, idleTimeout time.Duration, log logger.Logger) *connectionPool {
	p := &connectionPool{
		address:     address,
		maxConns:    maxConns,
		idleTimeout: idleTimeout,
		logger:      log,
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
			if err := conn.Close(); err != nil {
				p.logger.Errorf("Failed to close stale connection: %v", err)
			}
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
			if err := conn.Close(); err != nil {
				p.logger.Errorf("Failed to close overflow connection: %v", err)
			}
		}
	} else {
		// Bad connection or not pooled, close it
		if err := conn.Close(); err != nil {
			p.logger.Errorf("Failed to close bad connection: %v", err)
		}
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
	// Use ListenPacket to create an unconnected UDP socket
	// This allows receiving responses from any address (needed for NAT/Fly.io)
	conn, err := net.ListenPacket("udp", "")
	if err != nil {
		return nil, err
	}

	// Resolve the server address once
	serverAddr, err := net.ResolveUDPAddr("udp", p.address)
	if err != nil {
		conn.Close()
		return nil, err
	}

	p.mu.Lock()
	p.stats.Total++
	p.mu.Unlock()

	return &pooledConn{
		Conn:       &udpConn{PacketConn: conn, serverAddr: serverAddr},
		created:    time.Now(),
		lastUsed:   time.Now(),
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
				if err := conn.Close(); err != nil {
					p.logger.Errorf("Failed to close idle connection during cleanup: %v", err)
				}
			} else {
				// Put it back
				select {
				case p.conns <- conn:
				default:
					if err := conn.Close(); err != nil {
						p.logger.Errorf("Failed to close connection during cleanup: %v", err)
					}
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
		if err := conn.Close(); err != nil {
			p.logger.Errorf("Failed to close pooled connection during shutdown: %v", err)
		}
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
