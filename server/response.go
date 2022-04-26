package server

// Response is the response type for the server, it handles HMAC and other values
type Response struct {
	signature [64]byte
	hmac      [32]byte
	timestamp uint64
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
