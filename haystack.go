package haystack

import (
	"bufio"
	"net"
	"time"

	"github.com/nomasters/haystack/errors"
	"github.com/nomasters/haystack/needle"
	"github.com/nomasters/haystack/server"
)

const (
	// DefaultThreshold is the default time threshold
	// for a server response
	DefaultThreshold = time.Duration(3 * time.Second)
)

const (
	// ErrTimestampExceedsThreshold is an error returned with the timestamp exceeds the acceptable threshold
	ErrTimestampExceedsThreshold = errors.Error("Timestamp exceeds threshold")
)

type options struct {
}

type option func(*options)

// Client represents a haystack client with a UDP connection
type Client struct {
	raddr string
	// conn      net.Conn
	pubkey    *[32]byte
	preshared *[32]byte
	threshold time.Duration
}

// // Close implements the UDPConn.Close() method
// func (c *Client) Close() error {
// 	return c.conn.Close()
// }

// Set takes a needle and returns
func (c *Client) Set(n *needle.Needle) error {
	p := make([]byte, server.ResponseLength)
	conn, err := net.Dial("udp", c.raddr)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.Write(n.Bytes()); err != nil {
		return err
	}

	l, err := bufio.NewReader(conn).Read(p)
	if err != nil {
		return err
	}
	r, err := server.ResponseFromBytes(p[:l])

	if !validTimestamp(time.Now(), r.Timestamp(), c.threshold) {
		return ErrTimestampExceedsThreshold
	}

	return r.Validate(n.Hash(), c.pubkey, c.preshared)
}

// validTimestamp takes the current time, response claimed time, and acceptable threshold for time drift
// and returns a boolean. Drift is calculated in absolute terms.
func validTimestamp(now, claim time.Time, threshold time.Duration) bool {
	drift := now.Sub(claim)
	if drift < 0 {
		drift = -drift
	}
	return drift < threshold
}

// Get takes a needle hash and returns a Needle
func (c *Client) Get(h *needle.Hash) (*needle.Needle, error) {
	p := make([]byte, needle.NeedleLength)
	conn, err := net.Dial("udp", c.raddr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.Write(h[:])
	if _, err := bufio.NewReader(conn).Read(p); err != nil {
		return nil, err
	}
	return needle.FromBytes(p)
}

// NewClient creates a new haystack client. It requires an address
// but can also take an arbitrary number of options
func NewClient(address string, opts ...option) (*Client, error) {
	c := new(Client)
	c.raddr = address
	c.threshold = DefaultThreshold
	// conn, err := net.Dial("udp", address)
	// if err != nil {
	// 	return c, err
	// }
	// c.conn = conn
	return c, nil
}
