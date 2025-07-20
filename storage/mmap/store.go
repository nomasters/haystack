package mmap

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nomasters/haystack/logger"
	"github.com/nomasters/haystack/needle"
	"github.com/nomasters/haystack/storage"
)

// Store implements the storage.GetSetCloser interface using memory-mapped files.
type Store struct {
	config   *Config
	logger   logger.Logger
	dataFile *DataFile
	index    *Index
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
}

// Config holds configuration options for the memory-mapped storage.
type Config struct {
	// DataPath is the path to the data file
	DataPath string
	
	// IndexPath is the path to the index file
	IndexPath string
	
	// TTL is the time-to-live for stored needles
	TTL time.Duration
	
	// MaxItems is the maximum number of items to store
	MaxItems int
	
	// CompactThreshold triggers compaction when this fraction of records are deleted
	CompactThreshold float64
	
	// GrowthChunkSize is the size increment when growing files (bytes)
	GrowthChunkSize int64
	
	// SyncWrites forces synchronization after writes
	SyncWrites bool
	
	// CleanupInterval is how often to run TTL cleanup
	CleanupInterval time.Duration
	
	// Logger for error and info messages (optional, uses NoOp if nil)
	Logger logger.Logger
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig(basePath string) *Config {
	return &Config{
		DataPath:         basePath + ".haystack.data",
		IndexPath:        basePath + ".haystack.index",
		TTL:              24 * time.Hour,
		MaxItems:         2000000,
		CompactThreshold: 0.25,
		GrowthChunkSize:  1024 * 1024, // 1MB
		SyncWrites:       false,
		CleanupInterval:  2 * time.Hour,
		Logger:           nil, // Will use NoOp by default
	}
}

// New creates a new memory-mapped store with the given configuration.
func New(ctx context.Context, config *Config) (*Store, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	
	// Validate configuration
	if config.DataPath == "" || config.IndexPath == "" {
		return nil, fmt.Errorf("data and index paths must be specified")
	}
	
	if config.TTL <= 0 {
		return nil, fmt.Errorf("TTL must be positive")
	}
	
	if config.MaxItems <= 0 {
		return nil, fmt.Errorf("MaxItems must be positive")
	}
	
	// Use NoOp logger if none provided
	log := config.Logger
	if log == nil {
		log = logger.NewNoOp()
	}
	
	sctx, cancel := context.WithCancel(ctx)
	
	store := &Store{
		config: config,
		logger: log,
		ctx:    sctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	
	// Initialize data file
	dataFile, err := NewDataFile(config.DataPath, config.MaxItems, config.GrowthChunkSize)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create data file: %w", err)
	}
	store.dataFile = dataFile
	
	// Initialize index
	index, err := NewIndex(config.IndexPath, config.MaxItems)
	if err != nil {
		dataFile.Close()
		cancel()
		return nil, fmt.Errorf("failed to create index: %w", err)
	}
	store.index = index
	
	// Always rebuild index from data file on startup to ensure consistency
	if dataFile.header != nil && dataFile.header.RecordCount > 0 {
		// Clear the index first
		atomic.StoreUint32(&index.header.EntryCount, 0)
		
		if err := store.rebuildIndex(); err != nil {
			dataFile.Close()
			index.Close()
			cancel()
			return nil, fmt.Errorf("failed to rebuild index: %w", err)
		}
	}
	
	// Start background cleanup
	go store.cleanup()
	
	return store, nil
}

// Set stores a needle in the memory-mapped storage.
func (s *Store) Set(n *needle.Needle) error {
	if n == nil {
		return storage.ErrorNeedleIsNil
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	hash := n.Hash()
	expiration := time.Now().Add(s.config.TTL)
	
	// Check if needle already exists
	if offset, found := s.index.Find(hash); found {
		// Update existing record
		return s.dataFile.UpdateRecord(offset, n, expiration)
	}
	
	// Add new record
	offset, err := s.dataFile.AppendRecord(n, expiration)
	if err != nil {
		return fmt.Errorf("failed to append record: %w", err)
	}
	
	// Add to index
	if err := s.index.Insert(hash, offset); err != nil {
		// TODO: Consider rolling back the data file append
		return fmt.Errorf("failed to insert into index: %w", err)
	}
	
	if s.config.SyncWrites {
		if err := s.sync(); err != nil {
			return fmt.Errorf("failed to sync: %w", err)
		}
	}
	
	return nil
}

// Get retrieves a needle from the memory-mapped storage.
func (s *Store) Get(hash needle.Hash) (*needle.Needle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Find record in index
	offset, found := s.index.Find(hash)
	if !found {
		return nil, ErrDNE
	}
	
	// Read record from data file
	record, err := s.dataFile.ReadRecord(offset)
	if err != nil {
		return nil, fmt.Errorf("failed to read record: %w", err)
	}
	
	// Check if record is active and not expired
	if !record.IsActive() {
		return nil, ErrDNE
	}
	
	if time.Now().After(record.ExpirationTime()) {
		// Record is expired, mark as deleted lazily
		go s.markDeleted(offset)
		return nil, ErrDNE
	}
	
	return record.Needle(), nil
}

// Close closes the memory-mapped storage and releases resources.
func (s *Store) Close() error {
	s.cancel()
	<-s.done // Wait for cleanup goroutine to finish
	
	var errs []error
	
	if s.dataFile != nil {
		if err := s.dataFile.Close(); err != nil {
			errs = append(errs, fmt.Errorf("data file close: %w", err))
		}
	}
	
	if s.index != nil {
		if err := s.index.Close(); err != nil {
			errs = append(errs, fmt.Errorf("index close: %w", err))
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	
	return nil
}

// sync synchronizes memory-mapped changes to disk.
func (s *Store) sync() error {
	var errs []error
	
	if err := s.dataFile.Sync(); err != nil {
		errs = append(errs, fmt.Errorf("data file sync: %w", err))
	}
	
	if err := s.index.Sync(); err != nil {
		errs = append(errs, fmt.Errorf("index sync: %w", err))
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("sync errors: %v", errs)
	}
	
	return nil
}

// markDeleted marks a record as deleted asynchronously.
func (s *Store) markDeleted(offset uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if err := s.dataFile.MarkDeleted(offset); err != nil {
		s.logger.Errorf("Failed to mark record as deleted at offset %d: %v", offset, err)
	}
}

// cleanup runs periodic TTL cleanup and compaction.
func (s *Store) cleanup() {
	defer close(s.done)
	
	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.performCleanup()
		}
	}
}

// performCleanup removes expired records and triggers compaction if needed.
func (s *Store) performCleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Count active and deleted records
	stats := s.dataFile.GetStats()
	
	deletedRatio := float64(stats.DeletedRecords) / float64(stats.TotalRecords)
	
	// Trigger compaction if too many deleted records
	if deletedRatio > s.config.CompactThreshold {
		s.logger.Infof("Starting compaction: %.2f%% of records are deleted", deletedRatio*100)
		if err := s.compact(); err != nil {
			s.logger.Errorf("Failed to compact storage: %v", err)
			return
		}
		s.logger.Info("Compaction completed successfully")
	}
}

// compact rebuilds the data and index files to remove deleted records.
func (s *Store) compact() error {
	// Create new temporary files
	newDataPath := s.config.DataPath + ".compact"
	newIndexPath := s.config.IndexPath + ".compact"
	
	newDataFile, err := NewDataFile(newDataPath, s.config.MaxItems, s.config.GrowthChunkSize)
	if err != nil {
		return fmt.Errorf("failed to create new data file: %w", err)
	}
	defer func() {
		if newDataFile != nil {
			_ = newDataFile.Close() // Ignore error during cleanup
			_ = os.Remove(newDataPath) // Ignore error during cleanup
		}
	}()
	
	newIndex, err := NewIndex(newIndexPath, s.config.MaxItems)
	if err != nil {
		return fmt.Errorf("failed to create new index: %w", err)
	}
	defer func() {
		if newIndex != nil {
			_ = newIndex.Close() // Ignore error during cleanup
			_ = os.Remove(newIndexPath) // Ignore error during cleanup
		}
	}()
	
	// Copy active, non-expired records
	now := time.Now()
	s.index.ForEach(func(hash needle.Hash, offset uint64) bool {
		record, err := s.dataFile.ReadRecord(offset)
		if err != nil {
			return true // Continue with next record
		}
		
		if !record.IsActive() || now.After(record.ExpirationTime()) {
			return true // Skip deleted/expired records
		}
		
		// Copy to new files
		newOffset, err := newDataFile.AppendRecord(record.Needle(), record.ExpirationTime())
		if err != nil {
			return false // Stop on error
		}
		
		if err := newIndex.Insert(hash, newOffset); err != nil {
			return false // Stop on error
		}
		
		return true
	})
	
	// Sync new files
	if err := newDataFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync new data file: %w", err)
	}
	
	if err := newIndex.Sync(); err != nil {
		return fmt.Errorf("failed to sync new index: %w", err)
	}
	
	// Close old files (errors are not critical here since we're replacing them)
	if err := s.dataFile.Close(); err != nil {
		// Log error but continue with replacement
	}
	if err := s.index.Close(); err != nil {
		// Log error but continue with replacement
	}
	
	// Atomic replacement
	if err := os.Rename(newDataPath, s.config.DataPath); err != nil {
		return fmt.Errorf("failed to replace data file: %w", err)
	}
	
	if err := os.Rename(newIndexPath, s.config.IndexPath); err != nil {
		return fmt.Errorf("failed to replace index file: %w", err)
	}
	
	// Reopen files
	s.dataFile, err = NewDataFile(s.config.DataPath, s.config.MaxItems, s.config.GrowthChunkSize)
	if err != nil {
		return fmt.Errorf("failed to reopen data file: %w", err)
	}
	
	s.index, err = NewIndex(s.config.IndexPath, s.config.MaxItems)
	if err != nil {
		return fmt.Errorf("failed to reopen index: %w", err)
	}
	
	// Clear the defer cleanup since we successfully replaced
	newDataFile = nil
	newIndex = nil
	
	return nil
}

// rebuildIndex scans the data file and rebuilds the index.
func (s *Store) rebuildIndex() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	recordCount := s.dataFile.header.RecordCount
	
	for i := uint32(0); i < recordCount; i++ {
		// Safe conversion: DataHeaderSize is const, i is uint32, RecordSize is const
		offsetInt64 := DataHeaderSize + int64(i)*RecordSize
		if offsetInt64 < 0 {
			continue // Skip invalid offset
		}
		offset := uint64(offsetInt64)
		
		record, err := s.dataFile.ReadRecord(offset)
		if err != nil {
			continue // Skip invalid records
		}
		
		// Only index active records
		if record.IsActive() {
			needle := record.Needle()
			if needle != nil {
				hash := needle.Hash()
				if err := s.index.Insert(hash, offset); err != nil {
					return fmt.Errorf("failed to insert into index: %w", err)
				}
			}
		}
	}
	
	return nil
}