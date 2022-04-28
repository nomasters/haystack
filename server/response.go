package server

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/nomasters/haystack/needle"
	"golang.org/x/crypto/blake2b"
	sign "golang.org/x/crypto/nacl/sign"
)

const (
	sigLen         = sign.Overhead
	hashLen        = blake2b.Size256
	timeLen        = 8
	ResponseLength = sigLen + hashLen + timeLen
)

// Response is the response type for the server, it handles HMAC and other values
type Response struct {
	sig  [64]byte
	hash [32]byte
	exp  time.Time
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

// NewResponse takes a expiration, hashKey (needle.Hash), and optionally a preshared key and a privateKey.
// if the presharedKey is present, the mac is fed into an hmac with the presharedKey. If the privateKey is not nil,
// it signs the payload with the privateKey and the message which is the hash + exp concatenated.
func NewResponse(exp time.Time, hashKey needle.Hash, presharedKey *[64]byte, privateKey *[64]byte) (Response, error) {
	var sig [64]byte
	var hash [32]byte

	b := timeToBytes(exp)
	h := mac(hashKey, b)

	if presharedKey != nil {
		h = hmac(presharedKey, h)
	}
	m := append(h, b...)
	o := make([]byte, ResponseLength)
	// only run nacl Sign if not nil, otherwise let sig be an array of zeros
	if privateKey != nil {
		s := sign.Sign(o, m, privateKey)
		copy(sig[:], s)
	}
	copy(hash[:], h)
	r := Response{
		sig:  sig,
		hash: hash,
		exp:  exp,
	}
	return r, nil
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
	if len(b) != ResponseLength {
		return r, errors.New("invalid response length")
	}
	copy(r.sig[:], b[:64])
	copy(r.hash[:], b[64:96])
	r.exp = bytesToTime(b[96:])
	return r, nil
}

func timeToBytes(t time.Time) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(t.Unix()))
	return b
}

func bytesToTime(b []byte) time.Time {
	if len(b) != 8 {
		b = make([]byte, 8)
	}
	t := int64(binary.LittleEndian.Uint64(b))
	return time.Unix(t, 0)
}
