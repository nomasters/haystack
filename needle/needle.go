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
	ValueLength = MessageLength - KeyLength
)

var (
	ErrMessageTooShort = errors.New("message too short")
	ErrMessageTooLong  = errors.New("message too long")
	ErrInvalidHash     = errors.New("invalid hash")
)

type Eye [KeyLength]byte
type Shaft [ValueLength]byte

type Needle struct {
	eye   Eye
	shaft Shaft
}

// FromBytes takes a byte slice and expects it to be exactly the length of MessageLength.
func FromBytes(b []byte) (*Needle, error) {
	var e Eye
	var s Shaft
	if err := validateLength(len(b)); err != nil {
		return nil, err
	}
	copy(e[:], b[:KeyLength])
	copy(s[:], b[KeyLength:])
	n := Needle{eye: e, shaft: s}
	return &n, n.ValidateHash()
}

func (n Needle) Eye() []byte {
	return n.eye[:]
}

func (n Needle) Shaft() []byte {
	return n.shaft[:]
}

func (n Needle) Bytes() []byte {
	b := make([]byte, MessageLength)
	copy(b[:KeyLength], n.eye[:])
	copy(b[KeyLength:], n.shaft[:])
	return b
}

func (n Needle) ValidateHash() error {
	s := blake2b.Sum256(n.Shaft())
	if subtle.ConstantTimeCompare(s[:], n.Eye()) == 0 {
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
