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
	payload    [448]byte
	expiration time.Time
}

type cleanup struct {
	hash       [32]byte
	expiration time.Time
}

// Store is a struct that holds the in memory storage state
type Store struct {
	sync.RWMutex
	internal map[[32]byte]value
	ttl      time.Duration
	cleanups chan cleanup
	maxItems int
}

func (s *Store) Write(b []byte) (int, error) {
	n, err := needle.FromBytes(b)
	if err != nil {
		return 0, err
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
	// clean task to handle expiration
	s.cleanups <- cleanup{
		hash:       hash,
		expiration: expiration,
	}
	return needle.NeedleLength, nil
}

// Get takes a 32 byte hash and returns a pointer to a needle and an error
func (s *Store) Get(hash [32]byte) (*needle.Needle, error) {
	s.RLock()
	v, ok := s.internal[hash]
	s.RUnlock()
	if !ok {
		return nil, storage.ErrorDNE
	}
	return needle.New(v.payload)
}

// New returns a pointer to a Store
func New() *Store {
	s := Store{
		internal: make(map[[32]byte]value),
		ttl:      10 * time.Second,
		maxItems: 2000,
	}

	go func(s *Store) {
		s.cleanups = make(chan cleanup, s.maxItems*headroom)
		for {
			task := <-s.cleanups
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
	}(&s)

	return &s
}
