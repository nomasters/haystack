# Haystack Memory-Mapped Storage Design

## Overview

Custom memory-mapped storage implementation optimized for Haystack's fixed-size, content-addressed, TTL-based data model.

## File Format

### Data File (.haystack.data)

Fixed-size records for efficient memory mapping:

```
Record Layout (208 bytes total):
┌─────────────────┬──────────────┬─────────────┐
│ Needle (192B)   │ Expiration   │ Flags (8B)  │
│                 │ (8B)         │             │
└─────────────────┴──────────────┴─────────────┘
0                192            200           208
```

**Fields:**
- **Needle (192 bytes)**: Complete needle data (32B hash + 160B payload)
- **Expiration (8 bytes)**: Unix timestamp as int64 (nanoseconds)
- **Flags (8 bytes)**: Metadata and control bits

**Flag Bits:**
```
Bit 0: Active (1) / Deleted (0)
Bit 1-7: Reserved for future use
Bytes 1-7: Reserved for additional metadata
```

### Index File (.haystack.index)

Hash-to-offset mapping for O(1) lookups:

```
Index Entry (40 bytes):
┌─────────────────┬──────────────┐
│ Hash (32B)      │ Offset (8B)  │
└─────────────────┴──────────────┘
0                32             40
```

**Fields:**
- **Hash (32 bytes)**: SHA256 hash (needle key)
- **Offset (8 bytes)**: Byte offset in data file (uint64)

## Memory Layout

### Data File Memory Mapping
```go
type MappedRecord struct {
    Needle     [192]byte  // Direct needle data
    Expiration int64      // TTL timestamp  
    Flags      uint64     // Control flags
}
```

### Index File Memory Mapping
```go
type IndexEntry struct {
    Hash   [32]byte  // SHA256 hash key
    Offset uint64    // Data file offset
}
```

## Access Patterns

### Read Operation (Get)
1. Calculate hash (already provided)
2. Binary search index for hash → offset
3. Read record at offset from data file
4. Check active flag and expiration
5. Return needle if valid

### Write Operation (Set)
1. Calculate hash from needle
2. Check if hash exists in index
3. If exists: overwrite record, update expiration
4. If new: append to data file, add index entry
5. Update index file

### Delete Operation (Expire)
1. Find record offset via index
2. Mark record as deleted (clear active flag)
3. Keep index entry for now (cleanup during compaction)

## Concurrency Model

### Read-Write Coordination
- **Read operations**: Multiple concurrent readers via RWMutex.RLock()
- **Write operations**: Exclusive access via RWMutex.Lock()
- **Memory mapping**: Separate locks for data vs index files

### File Growth Strategy
- **Data file**: Append-only, grow by fixed chunks (e.g., 1MB)
- **Index file**: Rebuild when growth threshold reached (e.g., 75% full)
- **Atomic updates**: Write to temp files, then atomic rename

## TTL and Compaction

### Expiration Strategy
- **Background cleanup**: Goroutine runs every TTL/10 interval
- **Lazy deletion**: Mark expired records during access
- **Compaction trigger**: When deleted records exceed threshold (e.g., 25%)

### Compaction Process
1. Create new data and index files
2. Copy only active, non-expired records
3. Update in-memory mapping to new files
4. Atomic file replacement
5. Unmap and delete old files

## Error Recovery

### Crash Recovery
- **Consistency check**: Verify index points to valid data records
- **Partial write detection**: Check for incomplete records at file end
- **Automatic repair**: Rebuild index from data file if corrupted

### File Integrity
- **Magic headers**: File type identification
- **Checksums**: Optional record-level verification
- **Version compatibility**: Format version in headers

## Performance Optimizations

### Memory Management
- **Page alignment**: Align records to OS page boundaries where possible
- **Batch operations**: Group multiple writes to reduce mmap overhead
- **Read-ahead**: Leverage OS page cache for sequential access

### Index Optimization
- **Sorted index**: Keep index sorted by hash for binary search
- **Bloom filter**: Optional pre-filter for non-existent keys
- **Hash prefix**: Store only hash prefix in index if memory constrained

## Configuration Parameters

```go
type Config struct {
    DataFilePath     string        // Path to .haystack.data
    IndexFilePath    string        // Path to .haystack.index  
    TTL              time.Duration // Record expiration time
    MaxItems         int           // Maximum number of records
    CompactThreshold float64       // Trigger compaction (0.25 = 25% deleted)
    GrowthChunkSize  int64         // File growth increment (bytes)
    SyncWrites       bool          // Force sync after writes
}
```

## File Size Calculations

**For 2M records (default max):**
- Data file: 2M × 208 bytes = ~416 MB
- Index file: 2M × 40 bytes = ~80 MB  
- Total: ~496 MB maximum disk usage

**Growth characteristics:**
- Linear growth with number of records
- Predictable memory usage
- Efficient compaction (copy only active records)