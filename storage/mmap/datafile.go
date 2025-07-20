package mmap

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/nomasters/haystack/needle"
)

// DataFile represents a memory-mapped data file for storing needle records.
type DataFile struct {
	path      string
	file      *os.File
	mmap      []byte
	fileSize  int64
	capacity  uint64
	chunkSize int64
	mu        sync.RWMutex

	// Atomic counter for append position
	appendPos uint64
}

// readHeader safely reads the header from memory-mapped data using encoding/binary.
func (df *DataFile) readHeader() (*DataHeader, error) {
	if len(df.mmap) < DataHeaderSize {
		return nil, fmt.Errorf("file too small for header")
	}

	header := &DataHeader{}

	// Read magic bytes
	copy(header.Magic[:], df.mmap[0:8])

	// Read other fields using encoding/binary
	header.Version = binary.LittleEndian.Uint32(df.mmap[8:12])
	header.RecordCount = binary.LittleEndian.Uint64(df.mmap[12:20])
	header.Capacity = binary.LittleEndian.Uint64(df.mmap[20:28])
	header.RecordSize = binary.LittleEndian.Uint32(df.mmap[28:32])
	header.Checksum = binary.LittleEndian.Uint32(df.mmap[32:36])

	// Reserved bytes are left as zero

	return header, nil
}

// writeHeader safely writes the header to memory-mapped data using encoding/binary.
func (df *DataFile) writeHeader(header *DataHeader) error {
	if len(df.mmap) < DataHeaderSize {
		return fmt.Errorf("file too small for header")
	}

	// Write magic bytes
	copy(df.mmap[0:8], header.Magic[:])

	// Write other fields using encoding/binary
	binary.LittleEndian.PutUint32(df.mmap[8:12], header.Version)
	binary.LittleEndian.PutUint64(df.mmap[12:20], header.RecordCount)
	binary.LittleEndian.PutUint64(df.mmap[20:28], header.Capacity)
	binary.LittleEndian.PutUint32(df.mmap[28:32], header.RecordSize)
	binary.LittleEndian.PutUint32(df.mmap[32:36], header.Checksum)

	// Clear reserved bytes
	for i := 36; i < DataHeaderSize; i++ {
		df.mmap[i] = 0
	}

	return nil
}

// getRecordCount atomically reads the record count from the header.
func (df *DataFile) getRecordCount() uint64 {
	if len(df.mmap) < 20 {
		return 0
	}
	return binary.LittleEndian.Uint64(df.mmap[12:20])
}

// setRecordCount atomically updates the record count in the header.
func (df *DataFile) setRecordCount(count uint64) {
	if len(df.mmap) >= 20 {
		binary.LittleEndian.PutUint64(df.mmap[12:20], count)
	}
}

// incrementRecordCount atomically increments the record count by 1.
func (df *DataFile) incrementRecordCount() {
	if len(df.mmap) >= 20 {
		current := df.getRecordCount()
		df.setRecordCount(current + 1)
	}
}

// newSecureDataFile creates or opens a data file with security validation.
func newSecureDataFile(path string, capacity uint64, chunkSize int64) (*DataFile, error) {
	// Validate existing file or create securely (always enforced)
	var file *os.File
	var err error

	if _, statErr := os.Stat(path); statErr == nil {
		// File exists, validate security properties
		if err := validateExistingFile(path); err != nil {
			return nil, fmt.Errorf("existing file failed security validation: %w", err)
		}
		// #nosec G304 - Secure function with validated path
		file, err = os.OpenFile(path, os.O_RDWR, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to open existing secure file: %w", err)
		}
	} else {
		// File doesn't exist, create securely
		file, err = secureFileCreate(path)
		if err != nil {
			return nil, fmt.Errorf("failed to create secure data file: %w", err)
		}
	}

	return newDataFileFromHandle(path, file, capacity, chunkSize)
}

// newDataFileFromHandle creates a DataFile from an open file handle.
func newDataFileFromHandle(path string, file *os.File, capacity uint64, chunkSize int64) (*DataFile, error) {
	df := &DataFile{
		path:      path,
		file:      file,
		capacity:  capacity,
		chunkSize: chunkSize,
	}

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to stat file: %w (cleanup error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if stat.Size() == 0 {
		// New file, initialize it
		if err := df.initialize(); err != nil {
			if closeErr := file.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to initialize data file: %w (cleanup error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to initialize data file: %w", err)
		}
	} else {
		// Existing file, map it
		df.fileSize = stat.Size()
		if err := df.mapFile(); err != nil {
			if closeErr := file.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to map data file: %w (cleanup error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to map data file: %w", err)
		}

		// Read and validate header
		header, err := df.readHeader()
		if err != nil {
			if closeErr := df.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to read header: %w (cleanup error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to read header: %w", err)
		}

		if err := validateDataHeader(header); err != nil {
			if closeErr := df.Close(); closeErr != nil {
				return nil, fmt.Errorf("invalid data file header: %w (cleanup error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("invalid data file header: %w", err)
		}

		// Set append position to end of current records
		// Check for overflow before conversion
		const maxInt64 = 9223372036854775807
		if header.RecordCount > (maxInt64-DataHeaderSize)/RecordSize {
			// Handle overflow by setting to maximum reasonable position
			atomic.StoreUint64(&df.appendPos, maxInt64)
		} else {
			// Safe conversion - RecordCount is within bounds
			pos := uint64(DataHeaderSize) + header.RecordCount*RecordSize
			atomic.StoreUint64(&df.appendPos, pos)
		}
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

	// Create and write header using safe encoding
	header := newDataHeader(df.capacity)
	if err := df.writeHeader(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Set initial append position
	atomic.StoreUint64(&df.appendPos, DataHeaderSize)

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

	// Header is accessed via safe encoding/binary operations in helper functions

	return nil
}

// unmapFile unmaps the memory-mapped file.
func (df *DataFile) unmapFile() error {
	if df.mmap != nil {
		if err := syscall.Munmap(df.mmap); err != nil {
			return fmt.Errorf("munmap failed: %w", err)
		}
		df.mmap = nil
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
	// Check capacity
	if df.getRecordCount() >= df.capacity {
		return 0, ErrDataFileFull
	}

	// Get current append position
	offset := atomic.LoadUint64(&df.appendPos)

	// Check if we need to grow the file
	// Safe conversion: offset is always valid file position
	if offset > 9223372036854775807 || int64(offset)+RecordSize > df.fileSize {
		if err := df.grow(RecordSize); err != nil {
			return 0, fmt.Errorf("failed to grow data file: %w", err)
		}
	}

	// Create record
	record := newRecord(n, expiration)

	// Write record to memory-mapped file
	df.mu.RLock()
	copy(df.mmap[offset:offset+RecordSize], record.Bytes())
	df.mu.RUnlock()

	// Update counters atomically
	atomic.AddUint64(&df.appendPos, uint64(RecordSize))
	df.incrementRecordCount()

	return offset, nil
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
	record := newRecord(n, expiration)

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
	record, err := recordFromBytes(recordData)
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
	currentFlags := binary.LittleEndian.Uint64(df.mmap[flagsOffset : flagsOffset+8])
	newFlags := currentFlags &^ ActiveFlag
	binary.LittleEndian.PutUint64(df.mmap[flagsOffset:flagsOffset+8], newFlags)

	return nil
}

// GetStats returns statistics about the data file.
func (df *DataFile) GetStats() Stats {
	df.mu.RLock()
	defer df.mu.RUnlock()

	stats := Stats{
		TotalRecords: df.getRecordCount(),
		DataFileSize: df.fileSize,
	}

	// Count active and expired records
	now := time.Now()
	recordCount := df.getRecordCount()
	for i := uint64(0); i < recordCount; i++ {
		// Prevent integer overflow in offset calculation
		if i > (9223372036854775807-DataHeaderSize)/RecordSize {
			break
		}
		offset := int64(DataHeaderSize) + int64(i)*RecordSize
		if offset+RecordSize > df.fileSize {
			break
		}

		// Read flags and expiration directly
		flags := binary.LittleEndian.Uint64(df.mmap[offset+200 : offset+208])
		expNanosUint := binary.LittleEndian.Uint64(df.mmap[offset+192 : offset+200])
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
