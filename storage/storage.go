package storage

import (
	"time"

	"github.com/nomasters/haystack/needle"
)

const (
	// DefaultTTL of 1 day
	DefaultTTL = 24 * time.Hour
)

// Storage is a simple interface to interact with storage.
type Storage interface {
	Get(hash needle.Hash) (*needle.Needle, error)
	Set(needle *needle.Needle) error
}

// TODO: calculate user-friendly settings, including, how to base maxItems on memory size instead of count
// Think about the Storage API in general and think about other storage engines (such as Boltdb, redis, and dynamodb)
