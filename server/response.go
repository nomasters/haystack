package server

import (
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
	macLen      = hashLen + timeLen
	responseLen = sigLen + hashLen + timeLen
	timeOffset  = responseLen - timeLen
)

var (
	// ErrInvalidResponseLen is used if the byte slice doesn't match the expected length
	ErrInvalidResponseLen = errors.New("invalid response length")
)

// Response is the response type for the server, it handles HMAC and other values
type Response struct {
	internal [responseLen]byte
}

// WIP: the idea here is something like:
// p : payload is a uint64 encoded unix timestamp of expiration
// k : the needle key from the submitted payload
// s : the nacl sign signature
// h : hmac
// s(h|p)|h(k|len(p)|p)|p
// response.Validate(key, ...opts)
// for example:
// response.Validate(needle.Key(), WithHMAC(pubkey), WithSharedKey(sharedKey))
// this will make it easy to to ensure that a the basic response is correct
// while also allowing for additional features to be verified as well.

// s[64]h[32]p[8]

// func (r Response) Validate(hash needle.Hash, preshared []byte, pubkey []byte) {

// }

// NewResponse takes a timestamp, hashKey (needle.Hash), and optionally a preshared key and a privateKey.
// if the presharedKey is present, the mac is fed into an hmac with the presharedKey. If the privateKey is not nil,
// it signs the payload with the privateKey and the message which is the hash + timestamp concatenated.
func NewResponse(timestamp time.Time, hashKey needle.Hash, presharedKey *[64]byte, privateKey *[64]byte) (r Response) {
	ts := timeToBytes(timestamp)
	h := mac(hashKey, ts)
	if presharedKey != nil {
		h = hmac(presharedKey, h)
	}
	m := make([]byte, macLen)
	copy(m[:hashLen], h)
	copy(m[hashLen:], ts)
	o := make([]byte, responseLen)
	// create a random key if no privateKey is passed in
	// this way signed and unsigned messages look the same
	if privateKey == nil {
		var pk [64]byte
		rand.Read(pk[:])
		privateKey = &pk
	}
	sign.Sign(o, m, privateKey)
	copy(r.internal[:], o)
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
func (r Response) Validate(hashKey needle.Hash, publicKey *[32]byte, presharedKey *[64]byte) error {
	return nil
}

func mac(key needle.Hash, message []byte) []byte {
	mac, _ := blake2b.New256(key[:])
	return mac.Sum(message)
}

func hmac(key *[64]byte, message []byte) []byte {
	mac, _ := blake2b.New256(key[32:])
	hmac, _ := blake2b.New256(key[:32])
	return hmac.Sum(mac.Sum(message))
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
