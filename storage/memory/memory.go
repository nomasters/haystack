package memory

import (
	"sync"
	"time"

	"github.com/nomasters/haystack/needle"
	"github.com/nomasters/haystack/storage"
)

const (
	headroom = 2
)

type value struct {
	payload    needle.Payload
	expiration time.Time
}

type cleanup struct {
	hash       needle.Hash
	expiration time.Time
}

// Store is a struct that holds the in memory storage state
type Store struct {
	sync.RWMutex
	internal map[needle.Hash]value
	ttl      time.Duration
	cleanups chan cleanup
	maxItems int
}

// Set takes a needle and writes it to the memory store.
func (s *Store) Set(n *needle.Needle) error {
	if n == nil {
		return storage.ErrorNeedleIsNil
	}
	hash := n.Hash()
	payload := n.Payload()
	expiration := time.Now().Add(s.ttl)
	s.Lock()
	s.internal[hash] = value{
		payload:    payload,
		expiration: expiration,
	}
	s.Unlock()
	s.cleanups <- cleanup{
		hash:       hash,
		expiration: expiration,
	}
	return nil
}

// Get takes a 32 byte hash and returns a pointer to a needle and an error
func (s *Store) Get(hash needle.Hash) (*needle.Needle, error) {
	s.RLock()
	v, ok := s.internal[hash]
	s.RUnlock()
	if !ok {
		return nil, needle.ErrorDNE
	}
	// TODO: this should be FromBytes, not New
	return needle.New(v.payload[:])
}

// Close is meant to conform to the GetSetCloser interface.
// TODO: put a proper cleanup step here
func (s *Store) Close() error { return nil }

// New returns a pointer to a Store
func New(ttl time.Duration, maxItems int) *Store {
	s := Store{
		internal: make(map[needle.Hash]value),
		ttl:      ttl,
		maxItems: maxItems,
	}
	s.cleanups = make(chan cleanup, s.maxItems*headroom)

	go func(s *Store) {
		for {
			select {
			case task := <-s.cleanups:
				for {
					if (len(s.cleanups) > s.maxItems) || (task.expiration.Before(time.Now())) {
						s.Lock()
						v := s.internal[task.hash]
						if v.expiration.Equal(task.expiration) {
							delete(s.internal, task.hash)
						}
						s.Unlock()
						break
					}
				}
			}
		}
	}(&s)

	return &s
}
