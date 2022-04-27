package server

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/nomasters/haystack/needle"
)

const (
	ResponseLength = 64 + 32 + 8
)

// Response is the response type for the server, it handles HMAC and other values
type Response struct {
	sig  [64]byte
	hmac [32]byte
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

func NewResponse(exp time.Time, hash needle.Hash, presharedKey []byte, privateKey []byte) (Response, error) {
	e := uint64(exp.Unix())

	r := Response{
		exp: exp,
	}
	return r, nil
}

func ResponseFromBytes(b []byte) (r Response, err error) {
	if len(b) != ResponseLength {
		return r, errors.New("invalid response length")
	}
	copy(r.sig[:], b[:64])
	copy(r.hmac[:], b[64:96])
	r.exp = bytesToTime(b[96:])
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
