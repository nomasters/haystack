package needle

import (
	"crypto/subtle"
	"errors"
	"math"

	"golang.org/x/crypto/blake2b"
)

const (
	// EyeLen is the length in bytes of the hash prefix in any message
	EyeLen = 32
	// ShaftLen is the length of the remaining bytes of the message.
	ShaftLen = 448
	// NeedleLen is the number of bytes required for a needle
	NeedleLen = EyeLen + ShaftLen
	// entropyThreshold is the threshold in which the entropy validation
	// fails
	entropyThreshold = 0.91
)

var (
	ErrMessageTooShort = errors.New("message too short")
	ErrMessageTooLong  = errors.New("message too long")
	ErrInvalidHash     = errors.New("invalid hash")
	ErrInvalidEntropy  = errors.New("entropy does meet the minimum threshold")
)

type Eye [EyeLen]byte
type Shaft [ShaftLen]byte

type Needle struct {
	n [NeedleLen]byte
}

func New(s Shaft) (*Needle, error) {
	var n Needle
	h := blake2b.Sum256(s[:])
	copy(n.n[:EyeLen], h[:])
	copy(n.n[EyeLen:], s[:])
	return &n, n.Validate()
}

// FromBytes takes a byte slice and expects it to be exactly the length of MessageLength.
func FromBytes(b []byte) (*Needle, error) {
	var n Needle
	if err := validateLength(len(b)); err != nil {
		return nil, err
	}
	copy(n.n[:], b)
	return &n, n.Validate()
}

func (n Needle) Eye() []byte {
	return n.n[:EyeLen]
}

func (n Needle) Shaft() []byte {
	return n.n[EyeLen:]
}

func (n Needle) Bytes() []byte {
	return n.n[:]
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

func (n Needle) validEntropy() bool {
	return entropy(n.n[EyeLen:]) > entropyThreshold
}

func (n Needle) validHash() bool {
	h := blake2b.Sum256(n.Shaft())
	return subtle.ConstantTimeCompare(h[:], n.Eye()) == 1
}

func validateLength(l int) error {
	if l < NeedleLen {
		return ErrMessageTooShort
	}
	if l > NeedleLen {
		return ErrMessageTooLong
	}
	return nil
}

// entropy runs Shannon's entropy algorith
// and returns a float64 score between 0 and 1
func entropy(b []byte) float64 {
	var entropy float64
	l := float64(len(b))
	freqMap := make(map[byte]float64)
	for _, v := range b {
		freqMap[v] += 1
	}
	for _, v := range freqMap {
		freq := v / l
		entropy += freq * math.Log2(freq)
	}
	return -entropy / 8
}
