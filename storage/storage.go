package storage

import (
	"time"

	"github.com/nomasters/haystack/needle"
)

const (
	// DefaultTTL of 1 day
	DefaultTTL = 24 * time.Hour
)

// Storage is a simple interface to interact with storage. There are only two methods, Get and Set. Get takes a needle.Hash which is
type Storage interface {
	Get(hash needle.Hash) (*needle.Needle, error)
	Set(needle *needle.Needle) error
}
