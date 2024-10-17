package haystack

import (
	"bufio"
	"errors"
	"net"

	"github.com/nomasters/haystack/needle"
)

// some more thought needs to go into this, we most likely need:
// - a connection pool, this connection pool should "self heal", allow for timeouts, and support the pooling feature to be well behaved
// - logger primitives, this most likely needs to carry over from server implementation
// - a way to process responses in a scalable way. This might be best to keep a local storage buffer keyed by the hash. more thought needs to go into this.

var (
	// ErrTimestampExceedsThreshold is an error returned with the timestamp exceeds the acceptable threshold
	ErrTimestampExceedsThreshold = errors.New("Timestamp exceeds threshold")
)

type options struct {
}

type option func(*options)

// Client represents a haystack client with a UDP connection
type Client struct {
	raddr string
	conn  net.Conn
}

// Close implements the UDPConn.Close() method
func (c *Client) Close() error {
	return c.conn.Close()
}

// Set takes a needle and returns
func (c *Client) Set(n *needle.Needle) error {
	conn, err := net.Dial("udp", c.raddr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write(n.Bytes())
	return err
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
	// TODO: Because this is connectionless, we should create a readbuffer for conn that writes to client storage interface
	// and then read from that client storage interface. This will make reading async calls that go really fast... faster.
	return needle.FromBytes(p)
}

// NewClient creates a new haystack client. It requires an address
// but can also take an arbitrary number of options
func NewClient(address string, opts ...option) (*Client, error) {
	c := new(Client)
	c.raddr = address
	conn, err := net.Dial("udp", address)
	if err != nil {
		return c, err
	}
	c.conn = conn
	return c, nil
}
