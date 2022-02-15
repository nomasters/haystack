package needle

import (
	"crypto/rand"
	"net/http"
	"testing"
)

func TestEntropy(t *testing.T) {

	freq := make(map[string]uint64)

	for i := 0; i < 1000000; i++ {

		payload := make([]byte, 448)
		rand.Read(payload[:])

		result := http.DetectContentType(payload)
		freq[result] += 1
		// _, err := New(payload[:])

		// if err != nil {
		// 	t.Error(err)
		// }

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

// Ideas for test cases:
// - set of pure random: should get results in expected distribution
// - set of encrypted payloads should get the same
// - specific results like clear text and other common byte encodes should fail entropy
// - TODO: research common encodings and/or compression that might pass, it would be interesting
