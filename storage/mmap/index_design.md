# Hash Index Design

## Index Structure Choice

**Sorted Array with Binary Search**

For Haystack's uniformly distributed SHA256 keys and typical size (2M records), a sorted array provides:
- **O(log n) lookups**: ~21 comparisons for 2M records
- **Simple memory mapping**: Direct array access
- **Efficient reconstruction**: Easy to rebuild during compaction
- **Predictable performance**: No hash collision edge cases

## Index File Layout

```
Index File (.haystack.index):
┌─────────────┬─────────────────────────────────────┐
│ Header      │ Sorted Index Entries               │
│ (64 bytes)  │ [Hash32][Offset8]...                │
└─────────────┴─────────────────────────────────────┘
0            64                                   EOF
```

### Header Format (64 bytes)
```go
type IndexHeader struct {
    Magic      [8]byte   // "HAYSTIDX" magic number
    Version    uint32    // Format version (1)
    EntryCount uint32    // Number of index entries
    Capacity   uint32    // Maximum entries before rebuild
    Checksum   uint32    // Header checksum (CRC32)
    Reserved   [40]byte  // Future expansion
}
```

### Index Entry Format (40 bytes)
```go
type IndexEntry struct {
    Hash   [32]byte  // SHA256 hash key
    Offset uint64    // Byte offset in data file
}
```

## In-Memory Index Structure

```go
type Index struct {
    file     *os.File              // Index file handle
    mmap     []byte                // Memory-mapped index file
    header   *IndexHeader          // Mapped header
    entries  []IndexEntry          // Mapped entry array
    capacity int                   // Maximum entries before rebuild
    mu       sync.RWMutex          // Read-write mutex
}
```

## Lookup Algorithm

### Binary Search Implementation
```go
func (idx *Index) Find(hash needle.Hash) (offset uint64, found bool) {
    idx.mu.RLock()
    defer idx.mu.RUnlock()
    
    left, right := 0, int(idx.header.EntryCount)
    
    for left < right {
        mid := (left + right) / 2
        
        cmp := bytes.Compare(hash[:], idx.entries[mid].Hash[:])
        switch {
        case cmp == 0:
            return idx.entries[mid].Offset, true
        case cmp < 0:
            right = mid
        default:
            left = mid + 1
        }
    }
    
    return 0, false
}
```

### Performance Characteristics
- **Time Complexity**: O(log n)
- **Space Complexity**: O(n)
- **Worst case**: 21 comparisons for 2M entries
- **Cache friendly**: Sequential memory access pattern

## Insert/Update Operations

### Insert Algorithm
```go
func (idx *Index) Insert(hash needle.Hash, offset uint64) error {
    idx.mu.Lock()
    defer idx.mu.Unlock()
    
    // Find insertion point
    pos := idx.findInsertPosition(hash)
    
    // Check if hash already exists
    if pos < int(idx.header.EntryCount) && 
       bytes.Equal(idx.entries[pos].Hash[:], hash[:]) {
        // Update existing entry
        idx.entries[pos].Offset = offset
        return nil
    }
    
    // Check capacity
    if idx.header.EntryCount >= idx.capacity {
        return ErrIndexFull
    }
    
    // Shift entries to make room
    copy(idx.entries[pos+1:], idx.entries[pos:idx.header.EntryCount])
    
    // Insert new entry
    idx.entries[pos] = IndexEntry{
        Hash:   hash,
        Offset: offset,
    }
    
    idx.header.EntryCount++
    return nil
}
```

## Index Growth Strategy

### Growth Triggers
- **75% capacity**: Start planning rebuild
- **90% capacity**: Force rebuild before next insert
- **Compaction**: Always rebuild with exact size needed

### Rebuild Process
1. **Create new index file**: Size = current_entries * 1.5
2. **Copy sorted entries**: Maintain sort order
3. **Update header**: New capacity, entry count
4. **Atomic swap**: Replace old index with new
5. **Cleanup**: Unmap and delete old index file

```go
func (idx *Index) Rebuild(newCapacity int) error {
    // Create new index file
    newIndex, err := createIndexFile(idx.path+".new", newCapacity)
    if err != nil {
        return err
    }
    
    // Copy existing entries (already sorted)
    copy(newIndex.entries[:idx.header.EntryCount], 
         idx.entries[:idx.header.EntryCount])
    newIndex.header.EntryCount = idx.header.EntryCount
    
    // Atomic replacement
    if err := os.Rename(idx.path+".new", idx.path); err != nil {
        return err
    }
    
    // Update current index
    idx.close()
    return idx.open(idx.path)
}
```

## Memory Mapping Strategy

### Mapping Approach
```go
func (idx *Index) mapFile() error {
    // Memory map entire file
    data, err := syscall.Mmap(
        int(idx.file.Fd()),
        0,
        int(idx.fileSize),
        syscall.PROT_READ|syscall.PROT_WRITE,
        syscall.MAP_SHARED,
    )
    if err != nil {
        return err
    }
    
    idx.mmap = data
    
    // Map header
    idx.header = (*IndexHeader)(unsafe.Pointer(&data[0]))
    
    // Map entries array
    entriesPtr := unsafe.Pointer(&data[64])
    idx.entries = (*[maxEntries]IndexEntry)(entriesPtr)[:idx.capacity]
    
    return nil
}
```

### Memory Safety
- **Bounds checking**: Verify all array accesses
- **Version compatibility**: Check magic number and version
- **Checksum validation**: Verify header integrity
- **Graceful degradation**: Fall back to file I/O if mmap fails

## Concurrent Access

### Read Operations
- **Multiple readers**: Use RWMutex.RLock()
- **No blocking**: Binary search is read-only
- **Memory barrier**: Ensure consistent view of sorted array

### Write Operations  
- **Exclusive access**: Use RWMutex.Lock()
- **Atomic updates**: Update entry count last
- **Memory sync**: Ensure writes are visible to readers

### File Synchronization
```go
func (idx *Index) Sync() error {
    // Sync memory-mapped changes to disk
    return syscall.Msync(idx.mmap, syscall.MS_SYNC)
}
```

## Error Handling

### Corruption Recovery
1. **Header corruption**: Rebuild from data file
2. **Entry corruption**: Verify hash ordering, rebuild if needed
3. **Size mismatch**: Truncate or extend to expected size
4. **Checksum failure**: Mark as corrupted, trigger rebuild

### Backup Strategy
- **Write-ahead logging**: Log index changes before applying
- **Shadow indexes**: Keep previous version until new one is verified
- **Atomic commits**: Use rename for atomic replacement

## Performance Optimizations

### Cache Optimization
- **Entry prefetching**: Read multiple entries per cache line
- **Hash prefix**: Compare hash prefix first (faster)
- **Branch prediction**: Structure comparisons for common cases

### Memory Layout
```go
// Pack entry for cache efficiency
type PackedIndexEntry struct {
    HashPrefix uint64  // First 8 bytes of hash
    Hash       [24]byte // Remaining hash bytes  
    Offset     uint64   // Data file offset
}
```

### Lookup Hints
- **Last accessed**: Cache last lookup for repeated access
- **Bloom filter**: Pre-filter non-existent keys (optional)
- **Range caching**: Cache hash ranges for faster initial bounds

## Configuration

```go
type IndexConfig struct {
    Path            string  // Index file path
    InitialCapacity int     // Starting capacity
    GrowthFactor    float64 // Growth multiplier (1.5)
    RebuildThreshold float64 // Rebuild when 75% full
    SyncWrites      bool    // Sync after writes
    UseMmap         bool    // Enable memory mapping
}
```