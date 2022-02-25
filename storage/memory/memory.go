package memory

import (
	"sync"
	"time"

	"github.com/nomasters/haystack/needle"
	"github.com/nomasters/haystack/storage"
)

// Store is a map of [32]byte key and a struct value
type Store struct {
	sync.RWMutex
	m   map[[32]byte][448]byte
	ttl time.Duration
}

func (s *Store) Write(b []byte) (int, error) {
	n, err := needle.FromBytes(b)
	if err != nil {
		return 0, err
	}

	var hash [32]byte
	var payload [448]byte
	copy(hash[:], n.Hash())
	copy(payload[:], n.Payload())
	s.Lock()
	s.m[hash] = payload
	s.Unlock()
	// clean up after ttl
	go func() {
		time.Sleep(s.ttl)
		s.Lock()
		delete(s.m, hash)
		s.Unlock()
	}()

	return needle.NeedleLength, nil
}

// Get takes a 32 byte hash and returns a pointer to a needle and an error
func (s *Store) Get(hash [32]byte) (*needle.Needle, error) {
	s.RLock()
	v, ok := s.m[hash]
	s.RUnlock()
	if !ok {
		return nil, storage.ErrorDNE
	}
	return needle.New(v[:])
}

// New returns a pointer to a Store
func New() *Store {
	return &Store{
		m:   make(map[[32]byte][448]byte),
		ttl: 5 * time.Second,
	}
}
