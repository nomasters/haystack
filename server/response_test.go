package server

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
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

func TestResponseFromBytes(t *testing.T) {
	t.Parallel()

	valid, _ := hex.DecodeString("f82e0ca0d2fb1da76da6caf36a9d0d2838655632e85891216dc8b545d8f141099c774f812598fb751781df4b31d245880ef438b18d19162dbdef2d6d7cdb4ea0a0927ad5a06e5b22aeed1838472032397aa2d4584e35a8b6b522b943f668b00eabc73aeeba69e04625dff923c46217151b94d3c358e937ade50977aca965aad224a5056300000000")
	invalidShort, _ := hex.DecodeString("0f56efcf30e923137d76665ad97882799266c1b3a487b141a97311a9ebb8750b5617f1a6d44da7064d7b8f860e17d3932fb65506dc9131efcdb5b986d934810592e3796200000000c09ff936ffbe99af795d994b5843149beadc20b0608cce6392e37962000000")
	invalidLong, _ := hex.DecodeString("f82e0ca0d2fb1da76da6caf36a9d0d2838655632e85891216dc8b545d8f141099c774f812598fb751781df4b31d245880ef438b18d19162dbdef2d6d7cdb4ea0a0927ad5a06e5b22aeed1838472032397aa2d4584e35a8b6b522b943f668b00eabc73aeeba69e04625dff923c46217151b94d3c358e937ade50977aca965aad224a505630000000000")

	testTable := []struct {
		input []byte
		err   error
	}{
		{valid, nil},
		{invalidShort, ErrInvalidResponseLen},
		{invalidLong, ErrInvalidResponseLen},
	}

	for _, test := range testTable {
		r, err := ResponseFromBytes(test.input)
		if test.err != nil {
			if err != test.err {
				t.Errorf("expected: %v, found: %v", test.err, err)
			}
		} else {
			if !bytes.Equal(test.input, r.Bytes()) {
				t.Errorf("expected:\t%x\nresult:\t%x\n", test.input, r.Bytes())
			}
		}
	}
}

func TestResponse(t *testing.T) {
	t.Parallel()
	t.Run("Validate", func(t *testing.T) {
		t.Parallel()
		b, _ := hex.DecodeString("f82e0ca0d2fb1da76da6caf36a9d0d2838655632e85891216dc8b545d8f14109")
		var h needle.Hash
		copy(h[:], b)

		pk, sk, _ := ed25519.GenerateKey(nil)
		var priv [64]byte
		var pubkey [32]byte

		copy(priv[:], sk)
		copy(pubkey[:], pk)

		var badPubkey [32]byte
		var badPreshared [64]byte

		var preshared [64]byte
		rand.Read(preshared[:])

		testTable := []struct {
			resp      Response
			hash      needle.Hash
			pubkey    *[32]byte
			preshared *[64]byte
			err       error
		}{
			{NewResponse(time.Now(), h, &preshared, &priv), h, &pubkey, &preshared, nil},
			{NewResponse(time.Now(), h, nil, &priv), h, &pubkey, nil, nil},
			{NewResponse(time.Now(), h, nil, nil), h, nil, nil, nil},
			{NewResponse(time.Now(), h, &preshared, nil), h, nil, &badPreshared, ErrInvalidMAC},
			{NewResponse(time.Now(), h, nil, &priv), h, &badPubkey, nil, ErrInvalidSig},
		}

		for _, test := range testTable {
			if err := test.resp.Validate(test.hash, test.pubkey, test.preshared); err != test.err {
				if !errors.Is(err, test.err) {
					t.Errorf("expected: %v, found: %v", test.err, err)
				}
			}
		}
	})
	t.Run("Bytes", func(t *testing.T) {
		t.Parallel()
		b, _ := hex.DecodeString("f82e0ca0d2fb1da76da6caf36a9d0d2838655632e85891216dc8b545d8f141099c774f812598fb751781df4b31d245880ef438b18d19162dbdef2d6d7cdb4ea0a0927ad5a06e5b22aeed1838472032397aa2d4584e35a8b6b522b943f668b00eabc73aeeba69e04625dff923c46217151b94d3c358e937ade50977aca965aad224a5056300000000")
		r, _ := ResponseFromBytes(b)
		if !bytes.Equal(b, r.Bytes()) {
			t.Errorf("expected:\t%x\nresult:\t%x\n", b, r.Bytes())
		}
	})
	t.Run("Timestamp", func(t *testing.T) {
		t.Parallel()
		b, _ := hex.DecodeString("f82e0ca0d2fb1da76da6caf36a9d0d2838655632e85891216dc8b545d8f141099c774f812598fb751781df4b31d245880ef438b18d19162dbdef2d6d7cdb4ea0a0927ad5a06e5b22aeed1838472032397aa2d4584e35a8b6b522b943f668b00eabc73aeeba69e04625dff923c46217151b94d3c358e937ade50977aca965aad224a5056300000000")
		r, _ := ResponseFromBytes(b)

		expected := time.Unix(1661314340, 0)
		if !r.Timestamp().Equal(expected) {
			t.Errorf("expected:\t%v\nresult:\t%v\n", expected, r.Timestamp())
		}
	})
}

func BenchmarkNewResponseBothKeys(b *testing.B) {
	p, _ := hex.DecodeString("40e4350b03d8b0c9e340321210b259d9a20b19632929b4a219254a4269c11f820c75168c6a91d309f4b134a7d715a5ac408991e1cf9415995053cf8a4e185dae22a06617ac51ebf7d232bc49e567f90be4db815c2b88ca0d9a4ef7a5119c0e592c88dfb96706e6510fb8a657c0f70f6695ea310d24786e6d980e9b33cf2665342b965b2391f6bb982c4c5f6058b9cba58038d32452e07cdee9420a8bd7f514e1")
	n, _ := needle.New(p)
	h := n.Hash()

	var priv [64]byte
	rand.Read(priv[:])
	var preshared [64]byte
	rand.Read(preshared[:])
	now := time.Now()

	for n := 0; n < b.N; n++ {
		NewResponse(now, h, &preshared, &priv)
	}
}

func BenchmarkNewResponseNoKeys(b *testing.B) {
	p, _ := hex.DecodeString("40e4350b03d8b0c9e340321210b259d9a20b19632929b4a219254a4269c11f820c75168c6a91d309f4b134a7d715a5ac408991e1cf9415995053cf8a4e185dae22a06617ac51ebf7d232bc49e567f90be4db815c2b88ca0d9a4ef7a5119c0e592c88dfb96706e6510fb8a657c0f70f6695ea310d24786e6d980e9b33cf2665342b965b2391f6bb982c4c5f6058b9cba58038d32452e07cdee9420a8bd7f514e1")
	n, _ := needle.New(p)
	h := n.Hash()
	now := time.Now()

	for n := 0; n < b.N; n++ {
		NewResponse(now, h, nil, nil)
	}
}

func BenchmarkResponseBytes(b *testing.B) {
	p, _ := hex.DecodeString("40e4350b03d8b0c9e340321210b259d9a20b19632929b4a219254a4269c11f820c75168c6a91d309f4b134a7d715a5ac408991e1cf9415995053cf8a4e185dae22a06617ac51ebf7d232bc49e567f90be4db815c2b88ca0d9a4ef7a5119c0e592c88dfb96706e6510fb8a657c0f70f6695ea310d24786e6d980e9b33cf2665342b965b2391f6bb982c4c5f6058b9cba58038d32452e07cdee9420a8bd7f514e1")
	n, _ := needle.New(p)
	r := NewResponse(time.Now(), n.Hash(), nil, nil)
	for n := 0; n < b.N; n++ {
		r.Bytes()
	}
}
