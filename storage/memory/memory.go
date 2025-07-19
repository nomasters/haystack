package memory

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/nomasters/haystack/needle"
	"github.com/nomasters/haystack/storage"
)

var (
	// ErrorStoreFull is used when the Set method receives a nil pointer
	ErrorStoreFull = errors.New("Store is full")
	// ErrorDNE is returned when a key/value par does not exist
	ErrorDNE = errors.New("Does Not Exist")
)

type value struct {
	payload    needle.Payload
	expiration time.Time
}


// Store is a struct that holds the in memory storage state
type Store struct {
	sync.RWMutex
	internal map[needle.Hash]value
	ttl      time.Duration
	maxItems int
	ctx      context.Context
	cancel   context.CancelFunc
}

// Set takes a needle and writes it to the memory store.
func (s *Store) Set(n *needle.Needle) error {
	if n == nil {
		return storage.ErrorNeedleIsNil
	}
	s.Lock()
	if len(s.internal) > s.maxItems {
		s.Unlock()
		return ErrorStoreFull
	}
	hash := n.Hash()
	expiration := time.Now().Add(s.ttl)
	s.internal[hash] = value{
		payload:    n.Payload(),
		expiration: expiration,
	}
	s.Unlock()

	return nil
}

// Get takes a 32 byte hash and returns a pointer to a needle and an error
func (s *Store) Get(hash needle.Hash) (*needle.Needle, error) {
	s.RLock()
	v, ok := s.internal[hash]
	s.RUnlock()
	if !ok {
		return nil, ErrorDNE
	}
	b := append(hash[:], v.payload[:]...)
	return needle.FromBytes(b)
}

// Close is meant to conform to the GetSetCloser interface.
func (s *Store) cleanupExpired() {
	now := time.Now()
	s.Lock()
	for hash, v := range s.internal {
		if now.After(v.expiration) {
			delete(s.internal, hash)
		}
	}
	s.Unlock()
}

func (s *Store) Close() error {
	s.cancel()
	return nil
}

// New returns a pointer to a Store
func New(ctx context.Context, ttl time.Duration, maxItems int) *Store {
	sctx, cancel := context.WithCancel(ctx)

	s := Store{
		internal: make(map[needle.Hash]value),
		ttl:      ttl,
		maxItems: maxItems,
		ctx:      sctx,
		cancel:   cancel,
	}

	go func() {
		ticker := time.NewTicker(ttl / 10)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.cleanupExpired()
			}
		}
	}()

	return &s
}
