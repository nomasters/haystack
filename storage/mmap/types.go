package mmap

import (
	"encoding/binary"
	"time"

	"github.com/nomasters/haystack/needle"
)

const (
	// File format constants
	DataMagic  = "HAYSTDAT" // Data file magic number
	IndexMagic = "HAYSTIDX" // Index file magic number
	
	// Version information
	FormatVersion = uint32(1)
	
	// Record sizes
	RecordSize      = 208 // 192 needle + 8 expiration + 8 flags
	IndexEntrySize  = 40  // 32 hash + 8 offset
	DataHeaderSize  = 64  // Data file header size
	IndexHeaderSize = 64  // Index file header size
	
	// Flag bits
	ActiveFlag = uint64(1 << 0) // Record is active (not deleted)
	
	// Default capacities
	DefaultMaxRecords = 2000000
	DefaultMaxIndex   = 2000000
)

// DataHeader represents the header of a data file.
type DataHeader struct {
	Magic       [8]byte   // "HAYSTDAT"
	Version     uint32    // Format version
	RecordCount uint32    // Number of records
	Capacity    uint32    // Maximum records
	RecordSize  uint32    // Size of each record (should be 208)
	Checksum    uint32    // Header checksum
	Reserved    [36]byte  // Future expansion
}

// IndexHeader represents the header of an index file.
type IndexHeader struct {
	Magic       [8]byte   // "HAYSTIDX"
	Version     uint32    // Format version
	EntryCount  uint32    // Number of index entries
	Capacity    uint32    // Maximum entries
	EntrySize   uint32    // Size of each entry (should be 40)
	Checksum    uint32    // Header checksum
	Reserved    [36]byte  // Future expansion
}

// Record represents a single record in the data file.
type Record struct {
	data []byte // Raw record data (208 bytes)
}

// NewRecord creates a new record from a needle and expiration time.
func NewRecord(n *needle.Needle, expiration time.Time) *Record {
	data := make([]byte, RecordSize)
	
	// Copy needle data (first 192 bytes)
	copy(data[0:192], n.Bytes())
	
	// Set expiration (next 8 bytes)
	expNanos := expiration.UnixNano()
	// Safely convert int64 to uint64, clamping negative values to 0
	if expNanos < 0 {
		binary.LittleEndian.PutUint64(data[192:200], 0)
	} else {
		binary.LittleEndian.PutUint64(data[192:200], uint64(expNanos))
	}
	
	// Set flags (last 8 bytes) - mark as active
	binary.LittleEndian.PutUint64(data[200:208], ActiveFlag)
	
	return &Record{data: data}
}

// RecordFromBytes creates a record from raw bytes.
func RecordFromBytes(data []byte) (*Record, error) {
	if len(data) != RecordSize {
		return nil, ErrInvalidRecord
	}
	
	// Make a copy to ensure we own the data
	recordData := make([]byte, RecordSize)
	copy(recordData, data)
	
	return &Record{data: recordData}, nil
}

// Needle returns the needle stored in this record.
func (r *Record) Needle() *needle.Needle {
	n, _ := needle.FromBytes(r.data[0:192])
	return n
}

// ExpirationTime returns the expiration time of this record.
func (r *Record) ExpirationTime() time.Time {
	expNanosUint := binary.LittleEndian.Uint64(r.data[192:200])
	// Safely convert uint64 to int64, clamping to max int64 to prevent overflow
	if expNanosUint > 9223372036854775807 { // math.MaxInt64
		return time.Unix(0, 9223372036854775807)
	}
	return time.Unix(0, int64(expNanosUint))
}

// Flags returns the flags field of this record.
func (r *Record) Flags() uint64 {
	return binary.LittleEndian.Uint64(r.data[200:208])
}

// IsActive returns true if the record is active (not deleted).
func (r *Record) IsActive() bool {
	return (r.Flags() & ActiveFlag) != 0
}

// MarkDeleted marks the record as deleted.
func (r *Record) MarkDeleted() {
	flags := r.Flags() &^ ActiveFlag // Clear active flag
	binary.LittleEndian.PutUint64(r.data[200:208], flags)
}

// UpdateExpiration updates the expiration time of this record.
func (r *Record) UpdateExpiration(expiration time.Time) {
	expNanos := expiration.UnixNano()
	// Safely convert int64 to uint64, clamping negative values to 0
	if expNanos < 0 {
		binary.LittleEndian.PutUint64(r.data[192:200], 0)
	} else {
		binary.LittleEndian.PutUint64(r.data[192:200], uint64(expNanos))
	}
}

// Bytes returns the raw bytes of this record.
func (r *Record) Bytes() []byte {
	return r.data
}

// IndexEntry represents a single entry in the index file.
type IndexEntry struct {
	Hash   [32]byte // SHA256 hash
	Offset uint64   // Offset in data file
}

// Stats provides statistics about the storage.
type Stats struct {
	TotalRecords   int64 // Total number of records
	ActiveRecords  int64 // Number of active records
	DeletedRecords int64 // Number of deleted records
	ExpiredRecords int64 // Number of expired records
	DataFileSize   int64 // Size of data file in bytes
	IndexFileSize  int64 // Size of index file in bytes
}

// NewDataHeader creates a new data file header.
func NewDataHeader(capacity uint32) *DataHeader {
	header := &DataHeader{
		Version:    FormatVersion,
		Capacity:   capacity,
		RecordSize: RecordSize,
	}
	copy(header.Magic[:], DataMagic)
	return header
}

// NewIndexHeader creates a new index file header.
func NewIndexHeader(capacity uint32) *IndexHeader {
	header := &IndexHeader{
		Version:   FormatVersion,
		Capacity:  capacity,
		EntrySize: IndexEntrySize,
	}
	copy(header.Magic[:], IndexMagic)
	return header
}

// ValidateDataHeader validates a data file header.
func ValidateDataHeader(header *DataHeader) error {
	if string(header.Magic[:]) != DataMagic {
		return ErrCorruptedFile
	}
	
	if header.Version != FormatVersion {
		return ErrIncompatibleVersion
	}
	
	if header.RecordSize != RecordSize {
		return ErrIncompatibleVersion
	}
	
	return nil
}

// ValidateIndexHeader validates an index file header.
func ValidateIndexHeader(header *IndexHeader) error {
	if string(header.Magic[:]) != IndexMagic {
		return ErrCorruptedFile
	}
	
	if header.Version != FormatVersion {
		return ErrIncompatibleVersion
	}
	
	if header.EntrySize != IndexEntrySize {
		return ErrIncompatibleVersion
	}
	
	return nil
}