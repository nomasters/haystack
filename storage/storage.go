package storage

import (
	"io"

	"github.com/nomasters/haystack/needle"
)

// Storage is a simple interface to interact with storage.
type Storage interface {
	io.Writer
	Get(hash [32]byte) (needle.Needle, error)
}
