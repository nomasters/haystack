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
	ErrMessageTooShort = errors.New("Message too short")
	ErrMessageTooLong  = errors.New("Message too long")
)

// ValidLength returns a boolean of true if n == KeyLength or MessageLength
func ValidLength(n int) bool {
	return n == KeyLength || n == MessageLength
}
