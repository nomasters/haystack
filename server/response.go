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
	macLen     = blake2b.Size256
	timeLen    = 8
	messageLen = hashLen + timeLen
	headerLen  = messageLen + sigLen

	// ResponseLength is needle Hash + sigLen + (h)mac length + timeLen
	ResponseLength = messageLen + sigLen + macLen

	// ErrInvalidResponseLen is used if the byte slice doesn't match the expected length
	ErrInvalidResponseLen = errors.Error("invalid response length")
	// ErrInvalidMAC is an error when the response hash doesn't match the derived hash
	ErrInvalidMAC = errors.Error("(h)mac failed validation")
	// ErrInvalidSig is an error used when the signature fails validation.
	ErrInvalidSig = errors.Error("signature failed validation")
	// ErrInvalidHash is an error used when the need hash doesn't match.
	ErrInvalidHash = errors.Error("signature failed validation")
)

// needleHash || timestamp || sig || mac

// Response is the response type for the server, it handles HMAC and other values
type Response struct{ internal [ResponseLength]byte }

// NewResponse takes a timestamp, needleHash (needle.Hash), and optionally a preshared key and a privateKey.
// if the presharedKey is present, the mac is fed into an hmac with the presharedKey. If the privateKey is not nil,
// it signs the payload with the privateKey and the message which is the hash + timestamp concatenated.
func NewResponse(timestamp time.Time, needleHash needle.Hash, presharedKey *[32]byte, privateKey *[64]byte) (r Response) {
	m := make([]byte, messageLen)
	s := make([]byte, 64)

	copy(m[:hashLen], needleHash[:])
	copy(m[hashLen:], timeToBytes(timestamp))

	h := mac(m, presharedKey)

	// sign if a privateKey is present, otherwise generate fake data and insert in the signing bytes
	if privateKey != nil {
		s = sign.Sign(nil, m, privateKey)
	} else {
		rand.Read(s)
	}
	copy(r.internal[:messageLen], m)
	copy(r.internal[messageLen:headerLen], s[:sigLen])
	copy(r.internal[headerLen:], h)

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
	return r.internal[hashLen:messageLen]
}

func (r Response) messageBytes() []byte {
	return r.internal[:messageLen]
}

func (r Response) macBytes() []byte {
	return r.internal[headerLen:]
}

func (r Response) sigBytes() []byte {
	m := make([]byte, sigLen+messageLen)
	copy(m[:sigLen], r.internal[messageLen:headerLen])
	copy(m[sigLen:], r.messageBytes())
	return m
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
	m := mac(r.messageBytes(), presharedKey)
	if subtle.ConstantTimeCompare(r.macBytes(), m) == 0 {
		return ErrInvalidMAC
	}
	if publicKey != nil {
		if _, validSig := sign.Open(nil, r.sigBytes(), publicKey); !validSig {
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

func mac(m []byte, psk *[32]byte) (h []byte) {
	if psk != nil {
		mac, _ := blake2b.New256(psk[:])
		mac.Write(m)
		return mac.Sum(nil)
	}
	b := blake2b.Sum256(m)
	return b[:]
}
