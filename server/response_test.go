package server

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"github.com/nomasters/haystack/needle"
)

func TestTimeToBytes(t *testing.T) {
	t.Parallel()
	ts := time.Unix(1580256000, 0)
	b := timeToBytes(ts)
	expected := []byte{0, 203, 48, 94, 0, 0, 0, 0}
	if bytes.Compare(b, expected) != 0 {
		t.Errorf("%v converted to %v, expected: %v", ts, b, expected)
	}
}

func TestBytesToTime(t *testing.T) {
	t.Parallel()

	testTable := []struct {
		input    []byte
		expected time.Time
	}{
		{[]byte{0, 203, 48, 94, 0, 0, 0, 0}, time.Unix(1580256000, 0)},
		{[]byte{0}, time.Unix(0, 0)},
	}

	for _, test := range testTable {
		ts := bytesToTime(test.input)
		if !ts.Equal(test.expected) {
			t.Errorf("%v converted to %v, expected: %v", test.input, ts, test.expected)
		}
	}
}

func BenchmarkNewResponse(b *testing.B) {
	p, _ := hex.DecodeString("40e4350b03d8b0c9e340321210b259d9a20b19632929b4a219254a4269c11f820c75168c6a91d309f4b134a7d715a5ac408991e1cf9415995053cf8a4e185dae22a06617ac51ebf7d232bc49e567f90be4db815c2b88ca0d9a4ef7a5119c0e592c88dfb96706e6510fb8a657c0f70f6695ea310d24786e6d980e9b33cf2665342b965b2391f6bb982c4c5f6058b9cba58038d32452e07cdee9420a8bd7f514e1")
	var payload needle.Payload
	copy(payload[:], p)
	n, _ := needle.New(payload)
	h := n.Hash()

	var priv [64]byte
	var preshared [64]byte
	rand.Read(priv[:])
	rand.Read(preshared[:])

	for n := 0; n < b.N; n++ {
		NewResponse(time.Now(), h, nil, &priv)
	}
}

func BenchmarkResponseBytes(b *testing.B) {
	p, _ := hex.DecodeString("40e4350b03d8b0c9e340321210b259d9a20b19632929b4a219254a4269c11f820c75168c6a91d309f4b134a7d715a5ac408991e1cf9415995053cf8a4e185dae22a06617ac51ebf7d232bc49e567f90be4db815c2b88ca0d9a4ef7a5119c0e592c88dfb96706e6510fb8a657c0f70f6695ea310d24786e6d980e9b33cf2665342b965b2391f6bb982c4c5f6058b9cba58038d32452e07cdee9420a8bd7f514e1")
	var payload needle.Payload
	copy(payload[:], p)
	n, _ := needle.New(payload)
	r := NewResponse(time.Now(), n.Hash(), nil, nil)
	for n := 0; n < b.N; n++ {
		r.Bytes()
	}
}
