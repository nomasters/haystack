package mmap

import "errors"

var (
	// ErrIndexFull is returned when the index has reached capacity
	ErrIndexFull = errors.New("index is full")

	// ErrDataFileFull is returned when the data file has reached capacity
	ErrDataFileFull = errors.New("data file is full")

	// ErrInvalidRecord is returned when a record is malformed
	ErrInvalidRecord = errors.New("invalid record")

	// ErrInvalidOffset is returned when an offset is out of bounds
	ErrInvalidOffset = errors.New("invalid offset")

	// ErrCorruptedFile is returned when file corruption is detected
	ErrCorruptedFile = errors.New("corrupted file")

	// ErrIncompatibleVersion is returned when file version is not supported
	ErrIncompatibleVersion = errors.New("incompatible file version")

	// ErrDNE is returned when a key does not exist
	ErrDNE = errors.New("does not exist")
)
