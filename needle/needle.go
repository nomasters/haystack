package needle

import (
	"bytes"
	"errors"

	"lukechampine.com/blake3"
)

const (
	// HashLength is the length in bytes of the hash prefix in any message
	HashLength = 32
	// PayloadLength is the length of the remaining bytes of the message.
	PayloadLength = 160
	// NeedleLength is the number of bytes required for a valid needle.
	NeedleLength = HashLength + PayloadLength
)

// Hash represents an array of length HashLength
type Hash [HashLength]byte

// Payload represents an array of length PayloadLength
type Payload [PayloadLength]byte

// Needle is a container for a 160 byte payload
// and a 32 byte blake3 hash of the payload.
type Needle struct {
	hash    Hash
	payload Payload
}

var (
	// ErrorDNE is returned when a key/value par does not exist
	ErrorDNE = errors.New("Does Not Exist")
	// ErrorInvalidHash is an error for in invalid hash
	ErrorInvalidHash = errors.New("invalid blake3-256 hash")
	// ErrorByteSliceLength is an error for an invalid byte slice length passed in to New or FromBytes
	ErrorByteSliceLength = errors.New("invalid byte slice length")
)

// New creates a Needle used for submitting a payload to a Haystack sever. It takes a Payload
// byte slice that is 160 bytes in length and returns a reference to a
// Needle and an error. The purpose of this function is to make it
// easy to create a new Needle from a payload. This function handles creating a blake3
// hash of the payload, which is used by the Needle to submit to a haystack server.
func New(payload []byte) (*Needle, error) {
	if len(payload) != PayloadLength {
		return nil, ErrorByteSliceLength
	}
	return &Needle{
		hash:    Hash(blake3.Sum256(payload)),
		payload: Payload(payload),
	}, nil
}

// FromBytes is intended convert raw bytes (from UDP or storage) into a Needle.
// It takes a byte slice and expects it to be exactly the length of NeedleLength.
// The byte slice should consist of the first 32 bytes being the blake3 hash of the
// payload and the payload bytes. This function verifies the length of the byte slice,
// copies the bytes into a private [192]byte array, and validates the Needle. It returns
// a reference to a Needle and an error.
func FromBytes(b []byte) (*Needle, error) {
	if len(b) != NeedleLength {
		return nil, ErrorByteSliceLength
	}
	n := Needle{
		hash:    Hash(b[:HashLength]),
		payload: Payload(b[HashLength:]),
	}
	if err := n.validate(); err != nil {
		return nil, err
	}
	return &n, nil
}

// Hash returns a copy of the bytes of the blake3 256 hash of the Needle payload.
func (n *Needle) Hash() Hash {
	return n.hash
}

// Payload returns a byte slice of the Needle payload
func (n *Needle) Payload() Payload {
	return n.payload
}

// Bytes returns a byte slice of the entire 192 byte hash + payload
func (n *Needle) Bytes() []byte {
	b := make([]byte, NeedleLength)
	copy(b, n.hash[:])
	copy(b[HashLength:], n.payload[:])
	return b
}

// validate checks that a Needle has a valid hash and that it meets the entropy
// threshold, it returns either nil or an error.
func (n *Needle) validate() error {
	if hash := blake3.Sum256(n.payload[:]); !bytes.Equal(n.hash[:], hash[:]) {
		return ErrorInvalidHash
	}
	return nil
}
