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

// - server:
// - check if read request or write request
// -- if read, message should be 32 bytes, take this and attempt to retreive from storage
// -- if write, message should be 480 bytes, validate the payload and by taking the hash of the Value and comparing to the key
// --- if valid, write the payload to storage
// --- return the the hash(write hash + TTL)
// - create a storage interface for reads and writes
