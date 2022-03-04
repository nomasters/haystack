package storage

import (
	"errors"
	"time"

	"github.com/nomasters/haystack/needle"
)

const (
	// DefaultTTL of 1 day
	DefaultTTL = 24 * time.Hour
)

var (
	// ErrorDNE is returned when a key/value par does not exist
	ErrorDNE = errors.New("Does Not Exist")
)

// Storage is a simple interface to interact with storage.
type Storage interface {
	Get(hash needle.Hash) (*needle.Needle, error)
	Set(needle *needle.Needle) error
}
