// Package descriptors provides thread-safe storage and retrieval of protocol buffer
// descriptors for the authorization service.
//
// Protocol buffer descriptors are binary files that describe the structure of protobuf
// messages and services. This package allows the authorization service to serve these
// descriptors to clients that need to dynamically understand the API schema.
//
// All operations are thread-safe using read-write locks, optimizing for the common case
// of many concurrent reads with infrequent writes.
package descriptors

import (
	"errors"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/logger"
	"os"
	"sync"
)

// Store manages protocol buffer descriptors in memory with thread-safe access.
// Descriptors are identified by name and stored as raw binary data.
type Store struct {
	descriptors map[string][]byte // Maps descriptor name to binary content
	mu          sync.RWMutex      // Protects concurrent access to descriptors map
	logger      *logger.Logger    // Logger for operational messages
}

// NewStore creates a new empty descriptor store.
func NewStore(log *logger.Logger) *Store {
	return &Store{
		descriptors: make(map[string][]byte),
		logger:      log,
	}
}

// LoadFromFile loads a protocol buffer descriptor from a file and stores it under the given name.
// The file is read entirely into memory for fast serving to clients.
//
// Parameters:
//   - name: The identifier for this descriptor (typically the filename)
//   - filePath: Absolute or relative path to the descriptor file
//
// Returns an error if the file cannot be read. The operation takes a write lock,
// so it should be called during initialization rather than during request handling.
func (s *Store) LoadFromFile(name, filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	s.descriptors[name] = data
	s.logger.Info("Loaded descriptor '%s' from %s (%d bytes)", name, filePath, len(data))
	return nil
}

// Get retrieves a descriptor by name.
// Returns the raw binary descriptor data and nil error if found,
// or nil data and an error if the descriptor doesn't exist.
//
// This operation uses a read lock, allowing many concurrent Get calls.
func (s *Store) Get(name string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.descriptors[name]
	if !ok {
		return nil, errors.New("descriptor not found")
	}

	return data, nil
}

// List returns the names of all stored descriptors.
// The returned slice is a new allocation and can be safely modified by the caller.
//
// This operation uses a read lock, allowing concurrent access.
func (s *Store) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.descriptors))
	for name := range s.descriptors {
		names = append(names, name)
	}
	return names
}

// Count returns the number of stored descriptors.
// This operation uses a read lock, allowing concurrent access.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.descriptors)
}
