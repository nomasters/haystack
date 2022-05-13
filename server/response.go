package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/rand"
	"time"

	"github.com/nomasters/haystack/needle"
	"golang.org/x/crypto/blake2b"
	sign "golang.org/x/crypto/nacl/sign"
)

const (
	sigLen      = sign.Overhead
	hashLen     = blake2b.Size256
	timeLen     = 8
	prefixLen   = sigLen + hashLen
	messageLen  = hashLen + timeLen
	responseLen = sigLen + hashLen + timeLen
	timeOffset  = responseLen - timeLen
)

var (
	// ErrInvalidResponseLen is used if the byte slice doesn't match the expected length
	ErrInvalidResponseLen = errors.New("invalid response length")
	// ErrInvalidMAC is an error when the response hash doesn't match the derived hash
	ErrInvalidMAC = errors.New("(h)mac failed validation")
	// ErrInvalidSig is an error used when the signature fails validation.
	ErrInvalidSig = errors.New("signature failed validation")
)

// Response is the response type for the server, it handles HMAC and other values
type Response struct {
	internal [responseLen]byte
}

// NewResponse takes a timestamp, needleHash (needle.Hash), and optionally a preshared key and a privateKey.
// if the presharedKey is present, the mac is fed into an hmac with the presharedKey. If the privateKey is not nil,
// it signs the payload with the privateKey and the message which is the hash + timestamp concatenated.
func NewResponse(timestamp time.Time, needleHash needle.Hash, presharedKey *[64]byte, privateKey *[64]byte) (r Response) {
	ts := timeToBytes(timestamp)
	h := mac(needleHash, ts)
	if presharedKey != nil {
		h = hmac(presharedKey, h)
	}
	m := make([]byte, messageLen)
	copy(m[:hashLen], h)
	copy(m[hashLen:], ts)
	if privateKey == nil {
		var pk [64]byte
		rand.Read(pk[:])
		privateKey = &pk
	}
	copy(r.internal[:], sign.Sign(nil, m, privateKey))
	return r
}

// Bytes returns a byte slice of a Response
func (r Response) Bytes() []byte {
	return r.internal[:]
}

// Timestamp returns time.Time encoded timestamp
func (r Response) Timestamp() time.Time {
	return bytesToTime(r.internal[timeOffset:])
}

// Validate takes a hash and optionally a pubkey and presharedKey to validate the Response message.
// If no error is found, it returns nil.
func (r Response) Validate(needleHash needle.Hash, publicKey *[32]byte, presharedKey *[64]byte) error {
	h := mac(needleHash, r.internal[timeOffset:])
	if presharedKey != nil {
		h = hmac(presharedKey, h)
	}
	m := make([]byte, messageLen)
	copy(m[:hashLen], h)
	copy(m[hashLen:], r.internal[timeOffset:])

	if !bytes.Equal(r.internal[sigLen:], m) {
		return ErrInvalidMAC
	}
	if publicKey != nil {
		if _, validSig := sign.Open(nil, r.internal[:], publicKey); !validSig {
			return ErrInvalidSig
		}
	}
	return nil
}

func mac(key needle.Hash, message []byte) []byte {
	mac, _ := blake2b.New256(key[:])
	mac.Write(message)
	return mac.Sum(nil)
}

func hmac(key *[64]byte, message []byte) (b []byte) {
	mac, _ := blake2b.New256(key[32:])
	hmac, _ := blake2b.New256(key[:32])
	mac.Write(message)
	hmac.Write(mac.Sum(nil))
	return hmac.Sum(nil)
}

// ResponseFromBytes takes a byte slice and returns a Response and error
func ResponseFromBytes(b []byte) (r Response, err error) {
	if len(b) != responseLen {
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
