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

type task struct {
	hash       [32]byte
	expiration time.Time
}

// Store is a map of [32]byte key and a struct value
type Store struct {
	sync.RWMutex
	internal   map[[32]byte]value
	ttl        time.Duration
	deleteChan chan task
	maxItems   int
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
	// clean up after ttl
	s.deleteChan <- task{
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
		s.deleteChan = make(chan task, s.maxItems*headroom)
		for {
			task := <-s.deleteChan
			for {
				if (len(s.deleteChan) > s.maxItems) || (task.expiration.Before(time.Now())) {
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
