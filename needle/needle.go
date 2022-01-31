package needle

import (
	"crypto/subtle"
	"errors"

	"golang.org/x/crypto/blake2b"
)

const (
	// MessageLength is the number of bytes required for a write message
	MessageLength = 480
	// KeyLength is the length in bytes of the key prefix in any message
	KeyLength = 32
	// ValueLength is the length of the remaining bytes of the message after the KeyLength is subtracked out.
	ValueLength = KeyLength - MessageLength
)

var (
	ErrMessageTooShort = errors.New("message too short")
	ErrMessageTooLong  = errors.New("message too long")
	ErrInvalidHash     = errors.New("invalid hash")
)

type Needle struct {
	internal [MessageLength]byte
}

// FromBytes takes a byte slice and expects it to be exactly the length of MessageLength.
func FromBytes(b []byte) (*Needle, error) {
	var n Needle
	if err := validateLength(len(b)); err != nil {
		return nil, err
	}
	copy(n.internal[:], b)
	return &n, n.ValidateHash()
}

func (n Needle) Key() []byte {
	return n.internal[:KeyLength]
}

func (n Needle) Value() []byte {
	return n.internal[KeyLength:]
}

func (n Needle) Bytes() []byte {
	return n.internal[:]
}

func (n Needle) ValidateHash() error {
	s := blake2b.Sum256(n.Value())
	if subtle.ConstantTimeCompare(s[:], n.Key()) == 0 {
		return ErrInvalidHash
	}
	return nil
}

func validateLength(l int) error {
	if l < MessageLength {
		return ErrMessageTooShort
	}
	if l > MessageLength {
		return ErrMessageTooLong
	}
	return nil
}
