package needle

import (
	"crypto/subtle"
	"errors"
	"math"

	"golang.org/x/crypto/blake2b"
)

const (
	// EyeLen is the length in bytes of the key prefix in any message
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
	copy(e[:], b[:EyeLen])
	copy(s[:], b[EyeLen:])
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
	b := make([]byte, NeedleLen)
	copy(b[:EyeLen], n.eye[:])
	copy(b[EyeLen:], n.shaft[:])
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

func (n Needle) validEntropy() bool {
	return entropy(n.shaft) > entropyThreshold
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

// entropy runs Shannon's entropy on a needle Shaft
// this returns a number between 0 and 1
func entropy(s Shaft) float64 {
	// entropy
	var e float64
	// map of byte frequencies
	f := make(map[byte]float64)
	for _, b := range s {
		f[b] += 1
	}
	for _, v := range f {
		freq := v / ShaftLen
		e += freq * math.Log2(freq)
	}
	return -e / 8
}
