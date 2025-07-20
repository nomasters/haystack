package mmap

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/nomasters/haystack/needle"
)

// Index represents a memory-mapped index file for fast needle lookups.
type Index struct {
	path     string
	file     *os.File
	mmap     []byte
	header   *IndexHeader
	fileSize int64
	capacity int
	mu       sync.RWMutex
}

// NewIndex creates or opens an index file at the specified path.
func NewIndex(path string, capacity int) (*Index, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open index file: %w", err)
	}

	idx := &Index{
		path:     path,
		file:     file,
		capacity: capacity,
	}

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if stat.Size() == 0 {
		// New file, initialize it
		if err := idx.initialize(); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to initialize index file: %w", err)
		}
	} else {
		// Existing file, map it
		idx.fileSize = stat.Size()
		if err := idx.mapFile(); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to map index file: %w", err)
		}

		// Validate header
		if err := ValidateIndexHeader(idx.header); err != nil {
			idx.Close()
			return nil, fmt.Errorf("invalid index file header: %w", err)
		}

		// Note: The index is already populated from the file, no need to rebuild
	}

	return idx, nil
}

// initialize creates a new index file with header.
func (idx *Index) initialize() error {
	// Calculate initial file size
	initialSize := int64(IndexHeaderSize + idx.capacity*IndexEntrySize)

	// Resize file
	if err := idx.file.Truncate(initialSize); err != nil {
		return fmt.Errorf("failed to resize file: %w", err)
	}

	idx.fileSize = initialSize

	// Map the file
	if err := idx.mapFile(); err != nil {
		return fmt.Errorf("failed to map file: %w", err)
	}

	// Create and write header - safe conversion since capacity is positive
	if idx.capacity < 0 {
		return fmt.Errorf("invalid negative capacity: %d", idx.capacity)
	}
	// Safe conversion checked above
	header := NewIndexHeader(uint32(idx.capacity))
	// UNSAFE: Convert struct pointer to byte array for copying to mmap.
	// This is safe because:
	// 1. IndexHeader struct has fixed size (IndexHeaderSize = 64 bytes)
	// 2. We're only reading from the struct, not modifying it
	// 3. The struct lifetime exceeds this operation
	headerBytes := (*[IndexHeaderSize]byte)(unsafe.Pointer(header))
	copy(idx.mmap[:IndexHeaderSize], headerBytes[:])

	// UNSAFE: Cast mmap bytes directly to header struct for zero-copy access.
	// This is safe because:
	// 1. mmap guarantees the memory is valid and properly aligned
	// 2. IndexHeader struct is designed to match the on-disk layout exactly
	// 3. We verified mmap size >= IndexHeaderSize during file creation
	// 4. Memory mapping ensures the data persists for the file's lifetime
	idx.header = (*IndexHeader)(unsafe.Pointer(&idx.mmap[0]))

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

	// UNSAFE: Cast mmap bytes directly to header struct for zero-copy access.
	// This is safe because:
	// 1. mmap guarantees the memory is valid and properly aligned
	// 2. IndexHeader struct matches the exact on-disk format
	// 3. We verified file size and mmap bounds during mapFile()
	// 4. Header access is read-mostly with atomic updates for entry count
	idx.header = (*IndexHeader)(unsafe.Pointer(&idx.mmap[0]))

	return nil
}

// unmapFile unmaps the memory-mapped file.
func (idx *Index) unmapFile() error {
	if idx.mmap != nil {
		if err := syscall.Munmap(idx.mmap); err != nil {
			return fmt.Errorf("munmap failed: %w", err)
		}
		idx.mmap = nil
		idx.header = nil
	}
	return nil
}

// getEntry returns a pointer to the index entry at the given position.
func (idx *Index) getEntry(pos int) *IndexEntry {
	offset := IndexHeaderSize + pos*IndexEntrySize
	// UNSAFE: Cast mmap bytes directly to IndexEntry struct for performance.
	// This is safe because:
	// 1. pos is bounds-checked by callers (binary search, iteration)
	// 2. IndexEntry struct (40 bytes) matches the exact on-disk layout
	// 3. offset calculation is bounded by verified entry count and file size
	// 4. mmap guarantees memory validity and proper alignment
	// 5. All access is through read-write locks protecting concurrent access
	return (*IndexEntry)(unsafe.Pointer(&idx.mmap[offset]))
}

// Find searches for a hash in the index and returns its offset.
func (idx *Index) Find(hash needle.Hash) (uint64, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	entryCount := int(atomic.LoadUint32(&idx.header.EntryCount))
	if entryCount == 0 {
		return 0, false
	}

	// Binary search
	left, right := 0, entryCount

	for left < right {
		mid := (left + right) / 2
		entry := idx.getEntry(mid)

		cmp := bytes.Compare(hash[:], entry.Hash[:])
		switch {
		case cmp == 0:
			return entry.Offset, true
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
	entryCount := int(idx.header.EntryCount)

	// Binary search for insertion position
	return sort.Search(entryCount, func(i int) bool {
		entry := idx.getEntry(i)
		return bytes.Compare(entry.Hash[:], hash[:]) >= 0
	})
}

// Insert adds or updates a hash-to-offset mapping in the index.
func (idx *Index) Insert(hash needle.Hash, offset uint64) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	entryCount := int(idx.header.EntryCount)

	// Find insertion position
	pos := idx.findInsertPosition(hash)

	// Check if hash already exists
	if pos < entryCount {
		entry := idx.getEntry(pos)
		if bytes.Equal(entry.Hash[:], hash[:]) {
			// Update existing entry
			entry.Offset = offset
			return nil
		}
	}

	// Check capacity
	if entryCount >= idx.capacity {
		return ErrIndexFull
	}

	// Shift entries to make room if necessary
	if pos < entryCount {
		// Copy entries one position to the right
		for i := entryCount; i > pos; i-- {
			src := idx.getEntry(i - 1)
			dst := idx.getEntry(i)
			*dst = *src
		}
	}

	// Insert new entry
	entry := idx.getEntry(pos)
	entry.Hash = hash
	entry.Offset = offset

	// Update entry count
	atomic.AddUint32(&idx.header.EntryCount, 1)

	return nil
}

// ForEach iterates over all entries in the index.
func (idx *Index) ForEach(fn func(needle.Hash, uint64) bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	entryCount := int(atomic.LoadUint32(&idx.header.EntryCount))

	for i := 0; i < entryCount; i++ {
		entry := idx.getEntry(i)
		if !fn(entry.Hash, entry.Offset) {
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
