package needle

import (
	"crypto/subtle"

	"github.com/nomasters/haystack/errors"
	"golang.org/x/crypto/blake2b"
)

// Hash represents an array of length HashLength
type Hash [32]byte

// Payload represents an array of length PayloadLength
type Payload [160]byte

const (
	// HashLength is the length in bytes of the hash prefix in any message
	HashLength = len(Hash{})
	// PayloadLength is the length of the remaining bytes of the message.
	PayloadLength = len(Payload{})
	// NeedleLength is the number of bytes required for a valid needle.
	NeedleLength = HashLength + PayloadLength
	// ErrorDNE is returned when a key/value par does not exist
	ErrorDNE = errors.Error("Does Not Exist")
	// ErrorInvalidHash is an error for in invalid hash
	ErrorInvalidHash = errors.Error("invalid blake2b-256 hash")
	// ErrorByteSliceLength is an error for an invalid byte slice length passed in to New or FromBytes
	ErrorByteSliceLength = errors.Error("invalid byte slice length")
)

// Needle is an immutable container for a [192]byte array that containers a 160 byte payload
// and a 32 byte blake2b hash of the payload.
type Needle struct{ internal [NeedleLength]byte }

// New creates a Needle used for submitting a payload to a Haystack sever. It takes a Payload
// byte slice that is 160 bytes in length and returns a reference to a
// Needle and an error. The purpose of this function is to make it
// easy to create a new Needle from a payload. This function handles creating a blake2b
// hash of the payload, which is used by the Needle to submit to a haystack server.
func New(payload []byte) (*Needle, error) {
	if len(payload) != PayloadLength {
		return nil, ErrorByteSliceLength
	}
	var n Needle
	h := blake2b.Sum256(payload)
	copy(n.internal[:HashLength], h[:])
	copy(n.internal[HashLength:], payload)
	if err := n.validate(); err != nil {
		return nil, err
	}
	return &n, nil
}

// FromBytes is intended convert raw bytes (from UDP or storage) into a Needle.
// It takes a byte slice and expects it to be exactly the length of NeedleLength.
// The byte slice should consist of the first 32 bytes being the blake2b hash of the
// payload and the payload bytes. This function verifies the length of the byte slice,
// copies the bytes into a private [192]byte array, and validates the Needle. It returns
// a reference to a Needle and an error.
func FromBytes(b []byte) (*Needle, error) {
	if len(b) != NeedleLength {
		return nil, ErrorByteSliceLength
	}
	var n Needle
	copy(n.internal[:], b)
	if err := n.validate(); err != nil {
		return nil, err
	}
	return &n, nil
}

// Hash returns a copy of the bytes of the blake2b 256 hash of the Needle payload.
func (n Needle) Hash() Hash {
	var h Hash
	copy(h[:], n.internal[:HashLength])
	return h
}

// Payload returns a byte slice of the Needle payload
func (n Needle) Payload() Payload {
	var p Payload
	copy(p[:], n.internal[HashLength:])
	return p
}

// Bytes returns a byte slice of the entire 192 byte hash + payload
func (n Needle) Bytes() []byte {
	return n.internal[:]
}

// validate checks that a Needle has a valid hash and that it meets the entropy
// threshold, it returns either nil or an error.
func (n *Needle) validate() error {
	p := n.Payload()
	h := n.Hash()
	if hash := blake2b.Sum256(p[:]); subtle.ConstantTimeCompare(h[:], hash[:]) == 0 {
		return ErrorInvalidHash
	}
	return nil
}
