package server

import (
	"crypto/subtle"
	"encoding/binary"
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
	messageLen = hashLen + timeLen
	headerLen  = messageLen + sigLen

	// ResponseLength is needle hash || timestamp || sig || hash
	ResponseLength = messageLen + sigLen + hashLen

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
type Response struct{ internal [ResponseLength]byte }

// NewResponse takes a timestamp, needleHash (needle.Hash), and optionally a preshared key and a privateKey.
// if the presharedKey is present, the blake2b uses this key to make the hash a mac so that the client and server
// must use this key to properly validate the response. If the privateKey is not nil, it uses NaCl Sign for the payload
// the client must use the public key to verify the signature.
func NewResponse(timestamp time.Time, needleHash needle.Hash, presharedKey [32]byte, privateKey [64]byte) (r Response) {
	copy(r.internal[:hashLen], needleHash[:])
	copy(r.internal[hashLen:messageLen], timeToBytes(timestamp))
	copy(r.internal[messageLen:headerLen], sign.Sign(nil, r.internal[:messageLen], &privateKey))
	copy(r.internal[headerLen:], mac(r.internal[:headerLen], presharedKey))
	return
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

func (r Response) messageAndSigBytes() []byte {
	return r.internal[:headerLen]
}

func (r Response) sigBytes() []byte {
	m := make([]byte, headerLen)
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
func (r Response) Validate(needleHash needle.Hash, publicKey *[32]byte, presharedKey [32]byte) error {
	if subtle.ConstantTimeCompare(r.HashBytes(), needleHash[:]) == 0 {
		return ErrInvalidHash
	}
	m := mac(r.messageAndSigBytes(), presharedKey)
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

func mac(m []byte, psk [32]byte) (h []byte) {
	mac, _ := blake2b.New256(psk[:])
	mac.Write(m)
	return mac.Sum(nil)
}
