package storage

import (
	"errors"

	"github.com/nomasters/haystack/needle"
)

var (
	// ErrorNeedleIsNil is used when the Set method receives a nil pointer
	ErrorNeedleIsNil = errors.New("cannot set a nil *Needle")
)

// Getter takes a needle.Hash and returns a reference to needle.Needle and an error.
// The purpose is to provide a storage interface for getting a Needle by a Hash value
type Getter interface {
	Get(hash needle.Hash) (*needle.Needle, error)
}

// Setter takes a needle.Needle reference and returns an error
// The purpose is to write a needle to storage and return an error if any issues arise.
type Setter interface {
	Set(needle *needle.Needle) error
}

// Closer takes no arguments and returns an error.
// The purpose is to allow a signal to be passed to a storage
type Closer interface {
	Close() error
}

// GetSetCloser is the primary interface used by the haystack server, it allows for Getting, Setting, and Closings
type GetSetCloser interface {
	Getter
	Setter
	Closer
}
