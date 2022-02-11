package needle

import (
	"crypto/rand"
	"testing"
)

func TestEntropy(t *testing.T) {
	var a Shaft
	var b Shaft
	r1 := entropy(a)
	rand.Read(b[:])

	r2 := entropy(b)

	t.Log(r1)
	t.Log(r2)

}
