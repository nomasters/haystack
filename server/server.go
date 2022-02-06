package server

import (
	"github.com/nomasters/haystack/needle"
)

// this will be the primary processor for haystack server
// the idea here is to handle the two submit types
// but also sort out storage engines
// I think the primary ones should be:
// in memory, redis, dynamodb, and ssddb
// the interface should be simple, just a read and write aspect.

type Server struct {
	TTL      uint64
	Protocol string
	Storage  Storage
}

type Storage interface {
	Find(e needle.Eye) (needle.Needle, error)
	Write(n needle.Needle) error
	Delete(e needle.Eye) error
}

func New() (*Server, error) {
	s := Server{}
	return &s, nil
}
