package memory

import (
	"time"

	"github.com/nomasters/haystack/needle"
)

type value struct {
	ttl     time.Time
	payload [448]byte
}

// Store is a map of [32]byte key and a struct value
type Store map[[32]byte]value

func (s *Store) Write(b []byte) (n int, err error) {
	return
}

func (s Store) Get(hash [32]byte) (*needle.Needle, error) {
	return nil, nil
}
