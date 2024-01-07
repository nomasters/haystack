package memory

import (
	"context"
	"sync"
	"time"

	"github.com/nomasters/haystack/needle"
	"github.com/nomasters/haystack/storage"
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
		return storage.ErrorStoreFull
	}
	hash := n.Hash()
	expiration := time.Now().Add(s.ttl)
	s.internal[hash] = value{
		payload:    n.Payload(),
		expiration: expiration,
	}
	s.Unlock()

	go func() {
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(s.ttl):
			s.cleanups <- cleanup{hash: hash, expiration: expiration}
		}
	}()

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
	b := append(hash[:], v.payload[:]...)
	return needle.FromBytes(b)
}

// Close is meant to conform to the GetSetCloser interface.
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
		cleanups: make(chan cleanup, maxItems),
	}

	go func() {
		for {
			select {
			case <-s.ctx.Done():
				return
			case task := <-s.cleanups:
				s.Lock()
				v := s.internal[task.hash]
				if v.expiration.Equal(task.expiration) {
					delete(s.internal, task.hash)
				}
				s.Unlock()
			}
		}
	}()

	return &s
}
