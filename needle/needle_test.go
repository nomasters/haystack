package needle

import (
	"crypto/rand"
	"fmt"
	"testing"

	"golang.org/x/crypto/nacl/secretbox"
)

func TestEntropy(t *testing.T) {

	var key [32]byte
	rand.Read(key[:])
	message := []byte("Character is 100% free online character count calculator that's simple to use. Sometimes users prefer simplicity over all of the detailed writing information Word Counter provides, and this is exactly what this tool offers. It displays character count and word count which is often the only information a person needs to know about their writing. Best of all, you receive the needed33")

	freq := make(map[string]uint64)

	for i := 0; i < 10000; i++ {

		var nonce [24]byte
		rand.Read(nonce[:])
		var lookup [24]byte
		rand.Read(lookup[:])

		encrypted := secretbox.Seal(nonce[:], message, &nonce, &key)

		var s Shaft

		copy(s[:24], lookup[:])
		copy(s[24:], encrypted[:])

		e := entropy(s)
		k := fmt.Sprintf("%.2f", e)
		freq[k] += 1
	}

	// for i := 0; i < 100; i++ {
	// 	var s Shaft
	// 	rand.Read(s[:])
	// 	e := entropy(s)
	// 	k := fmt.Sprintf("%.2f", e)
	// 	freq[k] += 1
	// }
	t.Log(freq)
}
