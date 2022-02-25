package storage

import (
	"errors"
	"io"
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
	io.Writer
	Get(hash [32]byte) (*needle.Needle, error)
}
