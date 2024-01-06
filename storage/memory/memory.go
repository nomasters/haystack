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
		childCTX := context.WithoutCancel(s.ctx)
		select {
		case <-childCTX.Done():
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
// TODO: put a proper cleanup step here
func (s *Store) Close() error { return nil }

// todo: a better way to do this would be to not allow new items to be
// added to the store if it is full
// and we can use a return channel to signal a delete
// this would allow us to not have to do a cleanup on every set

// New returns a pointer to a Store
func New(ctx context.Context, ttl time.Duration, maxItems int) *Store {
	s := Store{
		internal: make(map[needle.Hash]value),
		ttl:      ttl,
		maxItems: maxItems,
		ctx:      ctx,
	}
	s.cleanups = make(chan cleanup, s.maxItems)

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
