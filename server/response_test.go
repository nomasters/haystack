package server

import (
	"bytes"
	"testing"
	"time"
)

func TestTimeToBytes(t *testing.T) {
	t.Parallel()
	ts, _ := time.Parse("2006-01-02", "2020-01-29")
	b := timeToBytes(ts)
	expected := []byte{0, 203, 48, 94, 0, 0, 0, 0}
	if bytes.Compare(b, expected) != 0 {
		t.Errorf("%v converted to %v, expected: %v", ts, b, expected)
	}
}
