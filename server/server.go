package server

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/nomasters/haystack/logger"
	"github.com/nomasters/haystack/needle"
	"github.com/nomasters/haystack/storage"
)

// Server implements a high-performance UDP server for Haystack operations.
// It combines transport and business logic for maximum efficiency.
type Server struct {
	storage  storage.GetSetCloser
	logger   logger.Logger
	conn     net.PacketConn
	bufPool  *sync.Pool
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
}

// Config holds configuration options for the Haystack server.
type Config struct {
	// Storage backend to use
	Storage storage.GetSetCloser
	// Logger for error and info messages (optional, uses NoOp if nil)
	Logger logger.Logger
}

// New creates a new Haystack UDP server with the given configuration.
func New(config *Config) *Server {
	if config == nil {
		config = &Config{}
	}
	
	// Use NoOp logger if none provided
	log := config.Logger
	if log == nil {
		log = logger.NewNoOp()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Server{
		storage: config.Storage,
		logger:  log,
		ctx:     ctx,
		cancel:  cancel,
		done:    make(chan struct{}),
		bufPool: &sync.Pool{
			New: func() any {
				// Pre-allocate buffer slightly larger than max packet size
				// to handle potential oversized packets gracefully
				return make([]byte, needle.NeedleLength+1)
			},
		},
	}
}

// ListenAndServe starts the UDP server on the given address.
// It blocks until an error occurs or the server is shut down.
func (s *Server) ListenAndServe(address string) error {
	if s.storage == nil {
		return fmt.Errorf("no storage configured")
	}
	
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}
	
	s.conn = conn
	
	// Start the main processing loop
	go s.serve()
	
	// Wait for shutdown
	<-s.done
	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	// Signal shutdown
	s.cancel()
	
	// Close the connection to unblock ReadFrom
	if s.conn != nil {
		if err := s.conn.Close(); err != nil {
			s.logger.Errorf("Failed to close connection during shutdown: %v", err)
		}
	}
	
	// Wait for serve loop to finish or timeout
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// serve is the main processing loop.
func (s *Server) serve() {
	defer close(s.done)
	defer func() {
		if s.conn != nil {
			if err := s.conn.Close(); err != nil {
				s.logger.Errorf("Failed to close connection during cleanup: %v", err)
			}
		}
	}()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			if err := s.processPacket(); err != nil {
				// In a production environment, you might want to log this
				// but continue processing other packets
				continue
			}
		}
	}
}

// processPacket handles a single UDP packet with minimal allocations.
func (s *Server) processPacket() error {
	// Get a buffer from the pool
	buf := s.bufPool.Get().([]byte)
	defer func() {
		// Reset buffer length and return to pool
		//nolint:staticcheck // SA6002: slice argument is intentional for buffer pools
		s.bufPool.Put(buf[:cap(buf)])
	}()
	
	// Set a read timeout to prevent blocking forever
	if err := s.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}
	
	n, addr, err := s.conn.ReadFrom(buf)
	if err != nil {
		// Check if it's a timeout error, which is expected
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil // Not an error, just a timeout
		}
		return err
	}
	
	// Process based on packet size
	switch n {
	case needle.HashLength:
		// GET operation: 32-byte hash query
		return s.handleGet(buf[:n], addr)
	case needle.NeedleLength:
		// SET operation: 192-byte needle storage
		return s.handleSet(buf[:n], addr)
	default:
		// Invalid packet size, silently drop
		return nil
	}
}

// handleGet processes a hash query and returns the corresponding needle.
func (s *Server) handleGet(hashBytes []byte, addr net.Addr) error {
	// Convert bytes to hash array
	var hash needle.Hash
	copy(hash[:], hashBytes)
	
	// Retrieve needle from storage
	n, err := s.storage.Get(hash)
	if err != nil {
		return fmt.Errorf("failed to get needle: %w", err)
	}
	
	// Send the full needle as response
	_, err = s.conn.WriteTo(n.Bytes(), addr)
	return err
}

// handleSet processes a needle storage request.
// No response is sent for SET operations (fire-and-forget for privacy).
func (s *Server) handleSet(needleBytes []byte, _ net.Addr) error {
	// Parse and validate the needle
	n, err := needle.FromBytes(needleBytes)
	if err != nil {
		return fmt.Errorf("invalid needle: %w", err)
	}
	
	// Store the needle
	if err := s.storage.Set(n); err != nil {
		return fmt.Errorf("failed to store needle: %w", err)
	}
	
	// No response for SET operations (by design)
	return nil
}