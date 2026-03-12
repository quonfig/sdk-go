package store

import (
	"sync"

	quonfig "github.com/quonfig/sdk-go"
)

// Store is a concurrency-safe in-memory config store.
type Store struct {
	mu      sync.RWMutex
	configs map[string]*quonfig.ConfigResponse
	version string
}

// New creates an empty Store.
func New() *Store {
	return &Store{
		configs: make(map[string]*quonfig.ConfigResponse),
	}
}

// Get returns a config by key, and whether it was found.
func (s *Store) Get(key string) (*quonfig.ConfigResponse, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.configs[key]
	return c, ok
}

// Update replaces all configs from the given envelope.
func (s *Store) Update(envelope *quonfig.ConfigEnvelope) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newConfigs := make(map[string]*quonfig.ConfigResponse, len(envelope.Configs))
	for i := range envelope.Configs {
		newConfigs[envelope.Configs[i].Key] = &envelope.Configs[i]
	}
	s.configs = newConfigs
	s.version = envelope.Meta.Version
}

// Keys returns all config keys.
func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.configs))
	for k := range s.configs {
		keys = append(keys, k)
	}
	return keys
}

// Version returns the current config version.
func (s *Store) Version() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

// Len returns the number of configs in the store.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.configs)
}
