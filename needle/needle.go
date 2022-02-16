package needle

import (
	"crypto/subtle"
	"fmt"
	"math"

	"golang.org/x/crypto/blake2b"
)

const (
	// HashLength is the length in bytes of the hash prefix in any message
	HashLength = 32
	// PayloadLength is the length of the remaining bytes of the message.
	PayloadLength = 448
	// NeedleLength is the number of bytes required for a valid needle.
	NeedleLength = HashLength + PayloadLength
	// EntropyThreshold is the minimum threshold of the payload's entropy allowed by the Needle validator
	EntropyThreshold = 0.90
)

type payload [PayloadLength]byte

// Needle is an immutable container for a [480]byte array that containers a 448 byte payload
// and a 32 byte blak2b hash of the payload.
type Needle struct {
	internal [NeedleLength]byte
}

// New creates a Needle used for submitting a payload to a Haystack sever. It takes a payload
// byte slice that expects to be exactly 448 bytes in length and returns a reference to a
// Needle and an error. The purpose of this function is to make it
// easy to create a new Needle from a payload. This function handles creating a blake2b
// hash of the payload, which is used by the Needle to submit to a haystack server.
func New(payload []byte) (*Needle, error) {
	var n Needle
	if err := validateLength(payload, PayloadLength); err != nil {
		return nil, err
	}
	h := blake2b.Sum256(payload)
	copy(n.internal[:HashLength], h[:])
	copy(n.internal[HashLength:], payload)
	return &n, n.validate()
}

// FromBytes is intended convert raw bytes (from UDP or storage) into a Needle.
// It takes a byte slice and expects it to be exactly the length of NeedleLength.
// The byteslice shoudl consist of the first 32 bytes being the blake2b hash of the
// payload and the payload bytes. This function verifies the length of the byte slice,
// copies the bytes into a private [480]byte array, and validates the Needle. It returns
// a reference to a Needle and an error.
func FromBytes(b []byte) (*Needle, error) {
	var n Needle
	if err := validateLength(b, NeedleLength); err != nil {
		return nil, err
	}
	copy(n.internal[:], b)
	return &n, n.validate()
}

// Hash returns a copy of the bytes of the blake2b 256 hash of the Needle payload.
func (n Needle) Hash() []byte {
	return n.internal[:HashLength]
}

// Payload returns a byte slice of the Needle payload
func (n Needle) Payload() []byte {
	return n.internal[HashLength:]
}

// Bytes returns a byte slice of the entire 480 byte hash + payload
func (n Needle) Bytes() []byte {
	return n.internal[:]
}

// Entropy is the Shannon Entropy score for the message payload.
func (n Needle) Entropy() float64 {
	var p payload
	copy(p[:], n.internal[HashLength:])
	return entropy(&p)
}

// validate checks that a Needle has a valid hash and that it meets the entropy
// threshold, it returns either nil or an error.
func (n *Needle) validate() error {
	if h := blake2b.Sum256(n.Payload()); subtle.ConstantTimeCompare(h[:], n.Hash()) == 0 {
		return fmt.Errorf("invalid blake2b-256 hash")
	}
	if score := n.Entropy(); score < EntropyThreshold {
		return fmt.Errorf("entropy score: %v, expected score > %v", score, EntropyThreshold)
	}
	return nil
}

// validateLength takes two arguments, a byte slice and its required
// length. It returns an error if the length is too long or too short.
func validateLength(b []byte, expected int) error {
	l := len(b)
	if l < expected {
		return fmt.Errorf("too few bytes, got: %v, expected: %v", l, expected)
	}
	if l > expected {
		return fmt.Errorf("too many bytes, got: %v, expected: %v", l, expected)
	}
	return nil
}

// entropy runs Shannon's entropy algorith
// and returns a float64 score between 0 and 1
func entropy(p *payload) float64 {
	var entropy float64
	freqMap := make(map[byte]float64)
	for _, v := range p {
		freqMap[v]++
	}
	for _, v := range freqMap {
		freq := v / PayloadLength
		entropy += freq * math.Log2(freq)
	}
	return -entropy / 8
}
