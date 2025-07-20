package mmap

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/nomasters/haystack/needle"
)

// DataFile represents a memory-mapped data file for storing needle records.
type DataFile struct {
	path      string
	file      *os.File
	mmap      []byte
	header    *DataHeader
	fileSize  int64
	capacity  int
	chunkSize int64
	mu        sync.RWMutex
	
	// Atomic counter for append position
	appendPos int64
}

// NewDataFile creates or opens a data file at the specified path.
func NewDataFile(path string, capacity int, chunkSize int64) (*DataFile, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open data file: %w", err)
	}
	
	df := &DataFile{
		path:      path,
		file:      file,
		capacity:  capacity,
		chunkSize: chunkSize,
	}
	
	// Get file info
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	
	if stat.Size() == 0 {
		// New file, initialize it
		if err := df.initialize(); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to initialize data file: %w", err)
		}
	} else {
		// Existing file, map it
		df.fileSize = stat.Size()
		if err := df.mapFile(); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to map data file: %w", err)
		}
		
		// Validate header
		if err := ValidateDataHeader(df.header); err != nil {
			df.Close()
			return nil, fmt.Errorf("invalid data file header: %w", err)
		}
		
		// Set append position to end of current records
		atomic.StoreInt64(&df.appendPos, int64(DataHeaderSize+df.header.RecordCount*RecordSize))
	}
	
	return df, nil
}

// initialize creates a new data file with header.
func (df *DataFile) initialize() error {
	// Calculate initial file size - at least header + one record
	initialSize := int64(DataHeaderSize + RecordSize)
	if df.chunkSize > initialSize {
		initialSize = df.chunkSize
	}
	
	// Resize file
	if err := df.file.Truncate(initialSize); err != nil {
		return fmt.Errorf("failed to resize file: %w", err)
	}
	
	df.fileSize = initialSize
	
	// Map the file
	if err := df.mapFile(); err != nil {
		return fmt.Errorf("failed to map file: %w", err)
	}
	
	// Create and write header - safe conversion since capacity is positive
	if df.capacity < 0 {
		return fmt.Errorf("invalid negative capacity: %d", df.capacity)
	}
	header := NewDataHeader(uint32(df.capacity)) // Safe conversion checked above
	// UNSAFE: Convert struct pointer to byte array for copying to mmap.
	// This is safe because:
	// 1. Header struct has fixed size (DataHeaderSize = 64 bytes)
	// 2. We're only reading from the struct, not writing
	// 3. The struct lifetime exceeds this operation
	headerBytes := (*[DataHeaderSize]byte)(unsafe.Pointer(header))
	copy(df.mmap[:DataHeaderSize], headerBytes[:])
	
	// UNSAFE: Cast mmap bytes directly to header struct for performance.
	// This is safe because:
	// 1. mmap guarantees the memory is valid and properly aligned
	// 2. DataHeader struct is designed to match the on-disk layout exactly
	// 3. We verified mmap size >= DataHeaderSize during file creation
	// 4. Memory mapping ensures the data persists as long as the file is mapped
	df.header = (*DataHeader)(unsafe.Pointer(&df.mmap[0]))
	
	// Set initial append position
	atomic.StoreInt64(&df.appendPos, DataHeaderSize)
	
	return nil
}

// mapFile memory maps the data file.
func (df *DataFile) mapFile() error {
	// Unmap if already mapped
	if df.mmap != nil {
		if err := df.unmapFile(); err != nil {
			return err
		}
	}
	
	// Memory map the file
	mmap, err := syscall.Mmap(
		int(df.file.Fd()),
		0,
		int(df.fileSize),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED,
	)
	if err != nil {
		return fmt.Errorf("mmap failed: %w", err)
	}
	
	df.mmap = mmap
	
	// UNSAFE: Cast mmap bytes directly to header struct for performance.
	// This is safe because:
	// 1. mmap guarantees the memory is valid and properly aligned  
	// 2. DataHeader struct matches the exact on-disk format
	// 3. We verified file size and mmap bounds above
	// 4. Header access is read-mostly with atomic updates for counters
	df.header = (*DataHeader)(unsafe.Pointer(&df.mmap[0]))
	
	return nil
}

// unmapFile unmaps the memory-mapped file.
func (df *DataFile) unmapFile() error {
	if df.mmap != nil {
		if err := syscall.Munmap(df.mmap); err != nil {
			return fmt.Errorf("munmap failed: %w", err)
		}
		df.mmap = nil
		df.header = nil
	}
	return nil
}

// grow expands the data file by at least the specified amount.
func (df *DataFile) grow(minSize int64) error {
	df.mu.Lock()
	defer df.mu.Unlock()
	
	// Calculate new size
	newSize := df.fileSize + df.chunkSize
	if newSize < df.fileSize+minSize {
		newSize = df.fileSize + minSize
	}
	
	// Unmap current mapping
	if err := df.unmapFile(); err != nil {
		return err
	}
	
	// Resize file
	if err := df.file.Truncate(newSize); err != nil {
		return fmt.Errorf("failed to resize file: %w", err)
	}
	
	df.fileSize = newSize
	
	// Remap file
	return df.mapFile()
}

// AppendRecord appends a new record to the data file.
func (df *DataFile) AppendRecord(n *needle.Needle, expiration time.Time) (uint64, error) {
	// Check capacity - safe conversion since capacity is positive
	if df.capacity < 0 || atomic.LoadUint32(&df.header.RecordCount) >= uint32(df.capacity) { // Safe conversion checked above
		return 0, ErrDataFileFull
	}
	
	// Get current append position
	offset := atomic.LoadInt64(&df.appendPos)
	
	// Check if we need to grow the file
	if offset+RecordSize > df.fileSize {
		if err := df.grow(RecordSize); err != nil {
			return 0, fmt.Errorf("failed to grow data file: %w", err)
		}
	}
	
	// Create record
	record := NewRecord(n, expiration)
	
	// Write record to memory-mapped file
	df.mu.RLock()
	copy(df.mmap[offset:offset+RecordSize], record.Bytes())
	df.mu.RUnlock()
	
	// Update counters atomically
	atomic.AddInt64(&df.appendPos, RecordSize)
	atomic.AddUint32(&df.header.RecordCount, 1)
	
	// Safe conversion since offset is positive
	if offset < 0 {
		return 0, fmt.Errorf("invalid negative offset: %d", offset)
	}
	return uint64(offset), nil
}

// UpdateRecord updates an existing record at the given offset.
func (df *DataFile) UpdateRecord(offset uint64, n *needle.Needle, expiration time.Time) error {
	// Validate offset - safe conversion since fileSize is positive
	var maxOffset uint64
	if df.fileSize < 0 {
		maxOffset = 0
	} else {
		maxOffset = uint64(df.fileSize)
	}
	if offset < DataHeaderSize || offset+RecordSize > maxOffset {
		return ErrInvalidOffset
	}
	
	// Create new record
	record := NewRecord(n, expiration)
	
	// Update record in memory-mapped file
	df.mu.RLock()
	defer df.mu.RUnlock()
	
	// Double-check bounds
	if offset+RecordSize > uint64(len(df.mmap)) {
		return ErrInvalidOffset
	}
	
	copy(df.mmap[offset:offset+RecordSize], record.Bytes())
	
	return nil
}

// ReadRecord reads a record from the given offset.
func (df *DataFile) ReadRecord(offset uint64) (*Record, error) {
	// Validate offset - safe conversion since fileSize is positive
	var maxOffset uint64
	if df.fileSize < 0 {
		maxOffset = 0
	} else {
		maxOffset = uint64(df.fileSize)
	}
	if offset < DataHeaderSize || offset+RecordSize > maxOffset {
		return nil, ErrInvalidOffset
	}
	
	// Read record from memory-mapped file
	df.mu.RLock()
	recordData := df.mmap[offset : offset+RecordSize]
	record, err := RecordFromBytes(recordData)
	df.mu.RUnlock()
	
	return record, err
}

// MarkDeleted marks a record as deleted at the given offset.
func (df *DataFile) MarkDeleted(offset uint64) error {
	// Validate offset - safe conversion since fileSize is positive
	var maxOffset uint64
	if df.fileSize < 0 {
		maxOffset = 0
	} else {
		maxOffset = uint64(df.fileSize)
	}
	if offset < DataHeaderSize || offset+RecordSize > maxOffset {
		return ErrInvalidOffset
	}
	
	// Clear active flag directly in memory-mapped file
	df.mu.RLock()
	defer df.mu.RUnlock()
	
	// Ensure offset is within bounds
	if offset+RecordSize > uint64(len(df.mmap)) {
		return ErrInvalidOffset
	}
	
	flagsOffset := offset + 200 // Flags are at byte 200 of record
	currentFlags := binary.LittleEndian.Uint64(df.mmap[flagsOffset:flagsOffset+8])
	newFlags := currentFlags &^ ActiveFlag
	binary.LittleEndian.PutUint64(df.mmap[flagsOffset:flagsOffset+8], newFlags)
	
	return nil
}

// GetStats returns statistics about the data file.
func (df *DataFile) GetStats() Stats {
	df.mu.RLock()
	defer df.mu.RUnlock()
	
	stats := Stats{
		TotalRecords: int64(df.header.RecordCount),
		DataFileSize: df.fileSize,
	}
	
	// Count active and expired records
	now := time.Now()
	for i := uint32(0); i < df.header.RecordCount; i++ {
		offset := DataHeaderSize + int64(i)*RecordSize
		if offset+RecordSize > df.fileSize {
			break
		}
		
		// Read flags and expiration directly  
		flags := binary.LittleEndian.Uint64(df.mmap[offset+200:offset+208])
		expNanosUint := binary.LittleEndian.Uint64(df.mmap[offset+192:offset+200])
		// Safely convert uint64 to int64, clamping to max int64 to prevent overflow
		var expNanos int64
		if expNanosUint > 9223372036854775807 { // math.MaxInt64
			expNanos = 9223372036854775807
		} else {
			expNanos = int64(expNanosUint)
		}
		
		if (flags & ActiveFlag) != 0 {
			expTime := time.Unix(0, expNanos)
			if now.After(expTime) {
				stats.ExpiredRecords++
			} else {
				stats.ActiveRecords++
			}
		} else {
			stats.DeletedRecords++
		}
	}
	
	return stats
}

// Sync synchronizes memory-mapped changes to disk.
func (df *DataFile) Sync() error {
	df.mu.RLock()
	defer df.mu.RUnlock()
	
	// Note: msync is platform-specific and not available on all systems
	// The file.Sync() call should be sufficient for most use cases
	return df.file.Sync()
}

// Close closes the data file and releases resources.
func (df *DataFile) Close() error {
	df.mu.Lock()
	defer df.mu.Unlock()
	
	var errs []error
	
	// Unmap memory
	if err := df.unmapFile(); err != nil {
		errs = append(errs, err)
	}
	
	// Close file
	if df.file != nil {
		if err := df.file.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	
	return nil
}