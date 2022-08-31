package server

import (
	"crypto/subtle"
	"encoding/binary"
	"math/rand"
	"time"

	"github.com/nomasters/haystack/errors"
	"github.com/nomasters/haystack/needle"
	"golang.org/x/crypto/blake2b"
	sign "golang.org/x/crypto/nacl/sign"
)

const (
	sigLen     = sign.Overhead
	hashLen    = blake2b.Size256
	timeLen    = 8
	headerLen  = hashLen + sigLen
	prefixLen  = headerLen + hashLen
	messageLen = hashLen + timeLen
	// ResponseLength is needle Hash + sigLen + (h)mac length + timeLen
	ResponseLength = prefixLen + timeLen

	// ErrInvalidResponseLen is used if the byte slice doesn't match the expected length
	ErrInvalidResponseLen = errors.Error("invalid response length")
	// ErrInvalidMAC is an error when the response hash doesn't match the derived hash
	ErrInvalidMAC = errors.Error("(h)mac failed validation")
	// ErrInvalidSig is an error used when the signature fails validation.
	ErrInvalidSig = errors.Error("signature failed validation")
	// ErrInvalidHash is an error used when the need hash doesn't match.
	ErrInvalidHash = errors.Error("signature failed validation")
)

// Response is the response type for the server, it handles HMAC and other values
type Response struct {
	internal [ResponseLength]byte
}

// NewResponse takes a timestamp, needleHash (needle.Hash), and optionally a preshared key and a privateKey.
// if the presharedKey is present, the mac is fed into an hmac with the presharedKey. If the privateKey is not nil,
// it signs the payload with the privateKey and the message which is the hash + timestamp concatenated.
func NewResponse(timestamp time.Time, needleHash needle.Hash, presharedKey *[32]byte, privateKey *[64]byte) (r Response) {
	m := messageWithHash(needleHash, timestamp, presharedKey)

	// sign if a privateKey is present, otherwise generate fake data and insert in the signing bytes
	if privateKey != nil {
		copy(r.internal[:], sign.Sign(needleHash[:], m, privateKey))
	} else {
		var b [64]byte
		rand.Read(b[:])
		copy(r.internal[:hashLen], needleHash[:])
		copy(r.internal[hashLen:headerLen], b[:])
		copy(r.internal[headerLen:], m)
	}

	return r
}

// Hash returns the needle Hash from the response
func (r Response) Hash() needle.Hash {
	var n needle.Hash
	copy(n[:], r.internal[:hashLen])
	return n
}

// HashBytes returns the needle Hash as a byte slice.
func (r Response) HashBytes() []byte {
	h := r.Hash()
	return h[:]
}

// Bytes returns a byte slice of a Response
func (r Response) Bytes() []byte {
	return r.internal[:]
}

func (r Response) timestampBytes() []byte {
	return r.internal[prefixLen:]
}

// Timestamp returns time.Time encoded timestamp
func (r Response) Timestamp() time.Time {
	return bytesToTime(r.timestampBytes())
}

// Validate takes a hash and optionally a pubkey and presharedKey to validate the Response message.
// If no error is found, it returns nil.
func (r Response) Validate(needleHash needle.Hash, publicKey *[32]byte, presharedKey *[32]byte) error {
	if subtle.ConstantTimeCompare(r.HashBytes(), needleHash[:]) == 0 {
		return ErrInvalidHash
	}

	m := messageWithHash(needleHash, r.Timestamp(), presharedKey)

	if subtle.ConstantTimeCompare(r.internal[headerLen:], m) == 0 {
		return ErrInvalidMAC
	}
	if publicKey != nil {
		if _, validSig := sign.Open(nil, r.internal[hashLen:], publicKey); !validSig {
			return ErrInvalidSig
		}
	}
	return nil
}

// ResponseFromBytes takes a byte slice and returns a Response and error
func ResponseFromBytes(b []byte) (r Response, err error) {
	if len(b) != ResponseLength {
		return r, ErrInvalidResponseLen
	}
	copy(r.internal[:], b)
	return r, nil
}

func timeToBytes(t time.Time) []byte {
	b := make([]byte, timeLen)
	binary.LittleEndian.PutUint64(b, uint64(t.Unix()))
	return b
}

func bytesToTime(b []byte) time.Time {
	if len(b) != timeLen {
		b = make([]byte, timeLen)
	}
	t := int64(binary.LittleEndian.Uint64(b))
	return time.Unix(t, 0)
}

func messageWithHash(needleHash needle.Hash, timestamp time.Time, presharedKey *[32]byte) []byte {
	var h []byte
	m := make([]byte, messageLen)

	copy(m[:hashLen], needleHash[:])
	copy(m[hashLen:], timeToBytes(timestamp))

	if presharedKey != nil {
		mac, _ := blake2b.New256(presharedKey[:])
		mac.Write(m)
		h = mac.Sum(nil)
	} else {
		b := blake2b.Sum256(m)
		h = b[:]
	}
	copy(m[:hashLen], h)
	return m
}
