package fixtures

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/quonfig/sdk-go/internal/eval"
)

// ConfigStore loads and stores all configs from integration-test-data for evaluation.
type ConfigStore struct {
	configs map[string]*eval.FullConfig
}

// NewConfigStore creates a new empty ConfigStore.
func NewConfigStore() *ConfigStore {
	return &ConfigStore{
		configs: make(map[string]*eval.FullConfig),
	}
}

// GetConfig returns a config by key.
func (s *ConfigStore) GetConfig(key string) (*eval.FullConfig, bool) {
	c, ok := s.configs[key]
	return c, ok
}

// Len returns the number of configs loaded.
func (s *ConfigStore) Len() int {
	return len(s.configs)
}

// LoadFromDir loads all config JSON files from a directory into the store.
func (s *ConfigStore) LoadFromDir(dir string) error {
	// Load configs, feature-flags, segments
	for _, subdir := range []string{"configs", "feature-flags", "segments"} {
		dirPath := filepath.Join(dir, subdir)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("reading directory %s: %w", dirPath, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}

			filePath := filepath.Join(dirPath, entry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("reading file %s: %w", filePath, err)
			}

			var cfg eval.FullConfig
			if err := json.Unmarshal(data, &cfg); err != nil {
				return fmt.Errorf("parsing config %s: %w", filePath, err)
			}

			s.configs[cfg.Key] = &cfg
		}
	}

	return nil
}
