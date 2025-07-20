# Unsafe vs Safe Index Implementation Performance Analysis

This document analyzes the performance differences between the unsafe pointer-based index implementation (`index.go`) and the safe encoding/binary-based implementation (`index_safe.go`).

## Benchmark Results

The following benchmarks were run on Apple M4 Pro with Go 1.24:

### Insert Performance (1000 operations per benchmark)

| Implementation | Time per op | Relative Performance | Memory Allocs |
|----------------|-------------|---------------------|---------------|
| **Unsafe**    | 208,421 ns  | **1.0x (baseline)** | 0 allocs/op   |
| **Safe**      | 1,015,428 ns| **4.9x slower**     | 0 allocs/op   |

**Analysis**: Safe implementation is ~5x slower for insert operations. This is primarily due to:
- Multiple `encoding/binary` calls per entry write
- Byte-by-byte field encoding instead of direct struct copying
- Additional bounds checking and validation

### Find Performance (1000 operations per benchmark)

| Implementation | Time per op | Relative Performance | Memory Allocs |
|----------------|-------------|---------------------|---------------|
| **Unsafe**    | 22,822 ns   | **1.0x (baseline)** | 0 allocs/op   |
| **Safe**      | 46,254 ns   | **2.0x slower**     | 0 allocs/op   |

**Analysis**: Safe implementation is ~2x slower for find operations. This is due to:
- `encoding/binary` field decoding for each entry read
- Creation of temporary `IndexEntry` structs instead of direct pointer access

### Mixed Operations (50% reads, 50% writes)

| Implementation | Time per op | Relative Performance | Memory Allocs |
|----------------|-------------|---------------------|---------------|
| **Unsafe**    | 249,493 ns  | **1.0x (baseline)** | 0 allocs/op   |
| **Safe**      | 1,275,766 ns| **5.1x slower**     | 0 allocs/op   |

**Analysis**: Mixed workload shows performance similar to insert-heavy benchmarks since writes dominate the performance impact.

### Initialization Performance

| Implementation | Time per op | Relative Performance | Memory Allocs |
|----------------|-------------|---------------------|---------------|
| **Unsafe**    | 166,423 ns  | **1.0x (baseline)** | 824 B, 8 allocs |
| **Safe**      | 169,761 ns  | **1.02x slower**    | 952 B, 9 allocs |

**Analysis**: Initialization performance is nearly identical (~2% difference). This makes sense since:
- File I/O dominates initialization time
- Header writing is done once vs many entry operations
- Memory mapping overhead is the same for both implementations

## Performance Impact Summary

### Operations Where Unsafe Provides Significant Benefits:
1. **Insert Operations**: 5x performance advantage
2. **Mixed Workloads**: 5x performance advantage  
3. **Find Operations**: 2x performance advantage

### Operations Where Performance Is Similar:
1. **Initialization**: <5% difference

## Memory Usage Analysis

Both implementations show:
- **Zero allocations** during steady-state operations (insert/find)
- **Identical memory footprint** for the actual data storage
- **Similar initialization cost** with safe version using slightly more temporary memory

## Safety vs Performance Trade-offs

### Unsafe Implementation Advantages:
- ✅ **5x faster writes** - Critical for high-throughput scenarios
- ✅ **2x faster reads** - Better for read-heavy workloads  
- ✅ **Zero-copy access** - Direct memory-mapped struct access
- ✅ **Lower CPU usage** - No encoding/decoding overhead

### Safe Implementation Advantages:
- ✅ **No unsafe operations** - Eliminates entire class of security issues
- ✅ **Portable** - No assumptions about struct layout or alignment
- ✅ **Easier to audit** - Standard library operations only
- ✅ **Future-proof** - Not dependent on Go's unsafe package stability

## Recommendations

### Use Unsafe Implementation When:
- High-performance scenarios (>10K ops/sec)
- Write-heavy workloads  
- Memory-mapped performance is critical
- Security audit can validate unsafe usage

### Use Safe Implementation When:
- Security is paramount over performance
- Compliance requires avoiding unsafe operations
- Performance requirements are modest (<1K ops/sec)
- Simplicity and maintainability are priorities

## Technical Implementation Differences

### Unsafe Implementation:
```go
// Direct pointer casting - zero overhead
entry := (*IndexEntry)(unsafe.Pointer(&idx.mmap[offset]))
return entry.Hash, entry.Offset
```

### Safe Implementation:
```go
// Encoding/binary operations - CPU overhead
var entry IndexEntry
copy(entry.Hash[:], idx.mmap[offset:offset+32])
entry.Offset = binary.LittleEndian.Uint64(idx.mmap[offset+32:offset+40])
return entry.Hash, entry.Offset
```

The performance difference directly reflects the overhead of the encoding/binary operations versus direct memory access.

## Conclusion

The benchmark results demonstrate a clear **5x performance penalty** for avoiding unsafe operations in write-heavy scenarios, with a **2x penalty** for reads. However, the safe implementation provides identical functionality and may be preferable in security-sensitive environments.

For Haystack's use case as a high-performance key-value store, the unsafe implementation provides significant advantages, but the safe implementation offers a viable alternative for scenarios where security considerations outweigh performance requirements.