package mmap

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"sync"
	"syscall"

	"github.com/nomasters/haystack/needle"
)

// Index represents a memory-mapped index file for fast needle lookups.
type Index struct {
	path     string
	file     *os.File
	mmap     []byte
	fileSize int64
	capacity uint64
	mu       sync.RWMutex
}

// readHeader safely reads the header from memory-mapped data using encoding/binary.
func (idx *Index) readHeader() (*IndexHeader, error) {
	if len(idx.mmap) < IndexHeaderSize {
		return nil, fmt.Errorf("file too small for header")
	}
	
	header := &IndexHeader{}
	
	// Read magic bytes
	copy(header.Magic[:], idx.mmap[0:8])
	
	// Read other fields using encoding/binary
	header.Version = binary.LittleEndian.Uint32(idx.mmap[8:12])
	header.EntryCount = binary.LittleEndian.Uint64(idx.mmap[12:20])
	header.Capacity = binary.LittleEndian.Uint64(idx.mmap[20:28])
	header.EntrySize = binary.LittleEndian.Uint32(idx.mmap[28:32])
	header.Checksum = binary.LittleEndian.Uint32(idx.mmap[32:36])
	
	// Reserved bytes are left as zero
	
	return header, nil
}

// writeHeader safely writes the header to memory-mapped data using encoding/binary.
func (idx *Index) writeHeader(header *IndexHeader) error {
	if len(idx.mmap) < IndexHeaderSize {
		return fmt.Errorf("file too small for header")
	}
	
	// Write magic bytes
	copy(idx.mmap[0:8], header.Magic[:])
	
	// Write other fields using encoding/binary
	binary.LittleEndian.PutUint32(idx.mmap[8:12], header.Version)
	binary.LittleEndian.PutUint64(idx.mmap[12:20], header.EntryCount)
	binary.LittleEndian.PutUint64(idx.mmap[20:28], header.Capacity)
	binary.LittleEndian.PutUint32(idx.mmap[28:32], header.EntrySize)
	binary.LittleEndian.PutUint32(idx.mmap[32:36], header.Checksum)
	
	// Clear reserved bytes
	for i := 36; i < IndexHeaderSize; i++ {
		idx.mmap[i] = 0
	}
	
	return nil
}

// getEntryCount atomically reads the entry count from the header.
func (idx *Index) getEntryCount() uint64 {
	if len(idx.mmap) < 20 {
		return 0
	}
	return binary.LittleEndian.Uint64(idx.mmap[12:20])
}

// setEntryCount atomically updates the entry count in the header.
func (idx *Index) setEntryCount(count uint64) {
	if len(idx.mmap) >= 20 {
		binary.LittleEndian.PutUint64(idx.mmap[12:20], count)
	}
}

// incrementEntryCount atomically increments the entry count by 1.
func (idx *Index) incrementEntryCount() {
	if len(idx.mmap) >= 20 {
		current := idx.getEntryCount()
		idx.setEntryCount(current + 1)
	}
}

// readEntry safely reads an index entry at the given position using encoding/binary.
func (idx *Index) readEntry(pos int) (needle.Hash, uint64, error) {
	offset := IndexHeaderSize + pos*IndexEntrySize
	if offset+IndexEntrySize > len(idx.mmap) {
		return needle.Hash{}, 0, fmt.Errorf("entry position out of bounds")
	}
	
	var hash needle.Hash
	copy(hash[:], idx.mmap[offset:offset+32])
	offsetValue := binary.LittleEndian.Uint64(idx.mmap[offset+32:offset+40])
	
	return hash, offsetValue, nil
}

// writeEntry safely writes an index entry at the given position using encoding/binary.
func (idx *Index) writeEntry(pos int, hash needle.Hash, offset uint64) error {
	entryOffset := IndexHeaderSize + pos*IndexEntrySize
	if entryOffset+IndexEntrySize > len(idx.mmap) {
		return fmt.Errorf("entry position out of bounds")
	}
	
	copy(idx.mmap[entryOffset:entryOffset+32], hash[:])
	binary.LittleEndian.PutUint64(idx.mmap[entryOffset+32:entryOffset+40], offset)
	
	return nil
}

// newSecureIndex creates or opens an index file with security validation.
func newSecureIndex(path string, capacity uint64) (*Index, error) {
	// Validate existing file or create securely (always enforced)
	var file *os.File
	var err error
	
	if _, statErr := os.Stat(path); statErr == nil {
		// File exists, validate security properties
		if err := validateExistingFile(path); err != nil {
			return nil, fmt.Errorf("existing index file failed security validation: %w", err)
		}
		// #nosec G304 - Secure function with validated path
		file, err = os.OpenFile(path, os.O_RDWR, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to open existing secure index file: %w", err)
		}
	} else {
		// File doesn't exist, create securely
		file, err = secureFileCreate(path)
		if err != nil {
			return nil, fmt.Errorf("failed to create secure index file: %w", err)
		}
	}
	
	return newIndexFromHandle(path, file, capacity)
}

// newIndexFromHandle creates an Index from an open file handle.
func newIndexFromHandle(path string, file *os.File, capacity uint64) (*Index, error) {
	idx := &Index{
		path:     path,
		file:     file,
		capacity: capacity,
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
		if err := idx.initialize(); err != nil {
			if closeErr := file.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to initialize index file: %w (cleanup error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to initialize index file: %w", err)
		}
	} else {
		// Existing file, map it
		idx.fileSize = stat.Size()
		if err := idx.mapFile(); err != nil {
			if closeErr := file.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to map index file: %w (cleanup error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to map index file: %w", err)
		}

		// Read and validate header
		header, err := idx.readHeader()
		if err != nil {
			if closeErr := idx.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to read header: %w (cleanup error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to read header: %w", err)
		}
		
		if err := validateIndexHeader(header); err != nil {
			if closeErr := idx.Close(); closeErr != nil {
				return nil, fmt.Errorf("invalid index file header: %w (cleanup error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("invalid index file header: %w", err)
		}

		// Note: The index is already populated from the file, no need to rebuild
	}

	return idx, nil
}

// initialize creates a new index file with header.
func (idx *Index) initialize() error {
	// Calculate initial file size
	// Safe conversion: capacity is uint64, IndexEntrySize is const int
	if idx.capacity > (1<<63-1)/IndexEntrySize {
		return fmt.Errorf("capacity too large: %d", idx.capacity)
	}
	initialSize := int64(IndexHeaderSize) + int64(idx.capacity)*IndexEntrySize

	// Resize file
	if err := idx.file.Truncate(initialSize); err != nil {
		return fmt.Errorf("failed to resize file: %w", err)
	}

	idx.fileSize = initialSize

	// Map the file
	if err := idx.mapFile(); err != nil {
		return fmt.Errorf("failed to map file: %w", err)
	}

	// Create and write header using safe encoding
	header := newIndexHeader(idx.capacity)
	if err := idx.writeHeader(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	return nil
}

// mapFile memory maps the index file.
func (idx *Index) mapFile() error {
	// Unmap if already mapped
	if idx.mmap != nil {
		if err := idx.unmapFile(); err != nil {
			return err
		}
	}

	// Memory map the file
	mmap, err := syscall.Mmap(
		int(idx.file.Fd()),
		0,
		int(idx.fileSize),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED,
	)
	if err != nil {
		return fmt.Errorf("mmap failed: %w", err)
	}

	idx.mmap = mmap

	// Header is accessed via safe encoding/binary operations in helper functions

	return nil
}

// unmapFile unmaps the memory-mapped file.
func (idx *Index) unmapFile() error {
	if idx.mmap != nil {
		if err := syscall.Munmap(idx.mmap); err != nil {
			return fmt.Errorf("munmap failed: %w", err)
		}
		idx.mmap = nil
	}
	return nil
}

// Find searches for a hash in the index and returns its offset.
func (idx *Index) Find(hash needle.Hash) (uint64, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	entryCount := idx.getEntryCount()
	if entryCount == 0 {
		return 0, false
	}

	// Binary search - safe conversion for array indexing
	if entryCount > 9223372036854775807 {
		return 0, false // Index too large for binary search
	}
	left, right := 0, int(entryCount)

	for left < right {
		mid := (left + right) / 2
		entryHash, entryOffset, err := idx.readEntry(mid)
		if err != nil {
			return 0, false
		}

		cmp := bytes.Compare(hash[:], entryHash[:])
		switch {
		case cmp == 0:
			return entryOffset, true
		case cmp < 0:
			right = mid
		default:
			left = mid + 1
		}
	}

	return 0, false
}

// findInsertPosition returns the position where a hash should be inserted.
func (idx *Index) findInsertPosition(hash needle.Hash) int {
	entryCount := idx.getEntryCount()

	// Binary search for insertion position - safe conversion for sort.Search
	if entryCount > 9223372036854775807 {
		return 9223372036854775807 // Return max safe int for very large indices
	}
	return sort.Search(int(entryCount), func(i int) bool {
		entryHash, _, err := idx.readEntry(i)
		if err != nil {
			return false
		}
		return bytes.Compare(entryHash[:], hash[:]) >= 0
	})
}

// Insert adds or updates a hash-to-offset mapping in the index.
func (idx *Index) Insert(hash needle.Hash, offset uint64) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	entryCount := idx.getEntryCount()

	// Find insertion position
	pos := idx.findInsertPosition(hash)

	// Check if hash already exists - safe conversion for comparison
	if entryCount > 9223372036854775807 {
		return ErrIndexFull // Index too large
	}
	entryCountInt := int(entryCount)
	if pos < entryCountInt {
		entryHash, _, err := idx.readEntry(pos)
		if err != nil {
			return fmt.Errorf("failed to read entry: %w", err)
		}
		if bytes.Equal(entryHash[:], hash[:]) {
			// Update existing entry
			if err := idx.writeEntry(pos, hash, offset); err != nil {
				return fmt.Errorf("failed to update entry: %w", err)
			}
			return nil
		}
	}

	// Check capacity
	if entryCount >= idx.capacity {
		return ErrIndexFull
	}

	// Shift entries to make room if necessary
	if pos < entryCountInt {
		// Copy entries one position to the right
		for i := entryCountInt; i > pos; i-- {
			srcHash, srcOffset, err := idx.readEntry(i - 1)
			if err != nil {
				return fmt.Errorf("failed to read source entry: %w", err)
			}
			if err := idx.writeEntry(i, srcHash, srcOffset); err != nil {
				return fmt.Errorf("failed to write destination entry: %w", err)
			}
		}
	}

	// Insert new entry
	if err := idx.writeEntry(pos, hash, offset); err != nil {
		return fmt.Errorf("failed to write new entry: %w", err)
	}

	// Update entry count
	idx.incrementEntryCount()

	return nil
}

// ForEach iterates over all entries in the index.
func (idx *Index) ForEach(fn func(needle.Hash, uint64) bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	entryCount := idx.getEntryCount()

	// Safe conversion for loop iteration
	if entryCount > 9223372036854775807 {
		return // Index too large to iterate
	}
	entryCountInt := int(entryCount)
	for i := 0; i < entryCountInt; i++ {
		entryHash, entryOffset, err := idx.readEntry(i)
		if err != nil {
			break // Skip invalid entries
		}
		if !fn(entryHash, entryOffset) {
			break
		}
	}
}

// Sync synchronizes memory-mapped changes to disk.
func (idx *Index) Sync() error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Note: msync is platform-specific
	return idx.file.Sync()
}

// Close closes the index file and releases resources.
func (idx *Index) Close() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	var errs []error

	// Unmap memory
	if err := idx.unmapFile(); err != nil {
		errs = append(errs, err)
	}

	// Close file
	if idx.file != nil {
		if err := idx.file.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	return nil
}
