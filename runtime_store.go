package quonfig

import "sync"

// runtimeStore is the SDK's in-memory config store for downloaded payloads.
type runtimeStore struct {
	mu      sync.RWMutex
	configs map[string]*ConfigResponse
	version string
}

func newRuntimeStore() *runtimeStore {
	return &runtimeStore{
		configs: make(map[string]*ConfigResponse),
	}
}

func (s *runtimeStore) Get(key string) (*ConfigResponse, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.configs[key]
	return cfg, ok
}

func (s *runtimeStore) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.configs))
	for key := range s.configs {
		keys = append(keys, key)
	}
	return keys
}

func (s *runtimeStore) Update(envelope *ConfigEnvelope) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := make(map[string]*ConfigResponse, len(envelope.Configs))
	for i := range envelope.Configs {
		next[envelope.Configs[i].Key] = &envelope.Configs[i]
	}

	s.configs = next
	s.version = envelope.Meta.Version
}
