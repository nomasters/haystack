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
	conn      net.Conn
	pubkey    *[32]byte
	preshared *[64]byte
	threshold time.Duration
}

// Close implements the UDPConn.Close() method
func (c *Client) Close() error {
	return c.conn.Close()
}

// Set takes a needle and returns
func (c *Client) Set(n *needle.Needle) error {
	p := make([]byte, server.ResponseLength)
	if _, err := c.conn.Write(n.Bytes()); err != nil {
		return err
	}

	l, err := bufio.NewReader(c.conn).Read(p)
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
	c.conn.Write(h[:])
	if _, err := bufio.NewReader(c.conn).Read(p); err != nil {
		return nil, err
	}
	return needle.FromBytes(p)
}

// NewClient creates a new haystack client. It requires an address
// but can also take an arbitrary number of options
func NewClient(address string, opts ...option) (*Client, error) {
	c := new(Client)
	c.threshold = DefaultThreshold
	conn, err := net.Dial("udp", address)
	if err != nil {
		return c, err
	}
	c.conn = conn
	return c, nil
}

// todo:
// - create hmac and verify functions
// - create get function
// - write tests

// TODO:
// setup haystack client:
// -- this should initialize a UDP config
// -- use client.Find()

// - server:
// - check if read request or write request
// -- if read, message should be 32 bytes, take this and attempt to retreive from storage
// -- if write, message should be 480 bytes, validate the payload and by taking the hash of the Value and comparing to the key
// --- if valid, write the payload to storage
// --- return the the hash(write hash + TTL)
// - create a storage interface for reads and writes

/*
Ideas for how the code works for the client.


// The client should configure a UDP connection and handle interacting with the server
client, err := haystack.Client("localhost:8080")

// if we want to pass in options, we can do it like this.
client, err := haystack.Client("localhost:8080", ...opts)

// with the client we can read and write to the haystack server
// it returns a needle and an error
needle, err := client.Get(key)

// posting to haystack takes a single argument of a needle
response, err := client.Post(needle)

*/

// response for posting a needle is:
// hash(posted hash + TTL in seconds) + submitted_key + TTL in seconds
// we should be able to verify that the hash is correct that the submitted key is correct and we should be able to use the TTL if needed
