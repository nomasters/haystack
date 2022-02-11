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
	ErrInvalidEntropy  = errors.New("entropy does meet the minimum threshold")
)

type Eye [KeyLength]byte
type Shaft [ValueLength]byte

type Needle struct {
	eye   Eye
	shaft Shaft
}

func New(s Shaft) *Needle {
	return &Needle{
		eye:   blake2b.Sum256(s[:]),
		shaft: s,
	}
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
	return &n, n.Validate()
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

func (n Needle) Validate() error {
	if !n.validHash() {
		return ErrInvalidHash
	}
	if !n.validEntropy() {
		return ErrInvalidEntropy
	}
	return nil
}

// TBD: measure entropy of the shaft and return a boolean value
func (n Needle) validEntropy() bool {
	return true
}

func (n Needle) validHash() bool {
	h := blake2b.Sum256(n.Shaft())
	return subtle.ConstantTimeCompare(h[:], n.Eye()) == 1
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
