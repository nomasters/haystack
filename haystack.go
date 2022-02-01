package haystack

import (
	"errors"
)

const (
	// MessageLength is the number of bytes required for a write message
	MessageLength = 480
	// BufferLength is the read buffer. This allows detecting messages that exceed the MessageLength.
	BufferLength = MessageLength + 1
	// KeyLength is the length in bytes of the key prefix in any message
	KeyLength = 32
	// ValueLength is the length of the remaining bytes of the message after the KeyLength is subtracked out.
	ValueLength = KeyLength - MessageLength
)

var (
	ErrMessageTooShort = errors.New("message too short")
	ErrMessageTooLong  = errors.New("message too long")
)

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
client, err := haystack.New("localhost:8080")

// if we want to pass in options, we can do it like this.
client, err := haystack.New("localhost:8080", ...opts)

// with the client we can read and write to the haystack server
// it returns a needle and an error
needle, err := client.Get(key)

// to create a needle you'd write, the message must be _exactly_ 448 bytes, or an error is returned
// this creates a needle which includes the blake2b 256 hash.
needle, err := needle.New(message)
// FromBytes creates a needle from 480 raw bytes.
needle, err := needle.FromBytes(b)

// posting to haystack takes a single argument of a needle
err := client.Post(needle)


*/
