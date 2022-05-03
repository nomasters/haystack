package server

import (
	"bytes"
	"testing"
	"time"
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
