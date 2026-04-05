package quonfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var workspaceSubdirectories = []string{
	"configs",
	"feature-flags",
	"segments",
	"log-levels",
	"schemas",
}

type workspaceConfig struct {
	ID              string        `json:"id"`
	ProjectID       string        `json:"projectId,omitempty"`
	Key             string        `json:"key"`
	Type            ConfigType    `json:"type"`
	ValueType       ValueType     `json:"valueType"`
	SendToClientSDK bool          `json:"sendToClientSdk"`
	Default         RuleSet       `json:"default"`
	Environments    []Environment `json:"environments,omitempty"`
}

func (c *workspaceConfig) FindEnvironment(envID string) *Environment {
	for i := range c.Environments {
		if c.Environments[i].ID == envID {
			return &c.Environments[i]
		}
	}
	return nil
}

func loadWorkspaceEnvelope(dir, environmentOverride string) (*ConfigEnvelope, error) {
	configs, err := loadWorkspaceConfigs(dir)
	if err != nil {
		return nil, err
	}

	environment, err := resolveWorkspaceEnvironment(dir, environmentOverride, configs)
	if err != nil {
		return nil, err
	}

	responses := make([]ConfigResponse, 0, len(configs))
	for i := range configs {
		cfg := configs[i]
		responses = append(responses, ConfigResponse{
			ID:              cfg.ID,
			Key:             cfg.Key,
			Type:            cfg.Type,
			ValueType:       cfg.ValueType,
			SendToClientSDK: cfg.SendToClientSDK,
			Default:         cfg.Default,
			Environment:     cfg.FindEnvironment(environment),
		})
	}

	return &ConfigEnvelope{
		Configs: responses,
		Meta: Meta{
			Version:     "local",
			Environment: environment,
			WorkspaceID: filepath.Base(filepath.Clean(dir)),
		},
	}, nil
}

func loadWorkspaceConfigs(dir string) ([]workspaceConfig, error) {
	var configs []workspaceConfig
	var errs []error

	for _, subdir := range workspaceSubdirectories {
		path := filepath.Join(dir, subdir)
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat %s: %w", path, err)
		}
		if !info.IsDir() {
			continue
		}

		err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if name := info.Name(); strings.HasPrefix(name, ".") && name != "." {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasPrefix(info.Name(), ".") || filepath.Ext(info.Name()) != ".json" {
				return nil
			}

			cfg, err := loadWorkspaceConfigFile(filePath)
			if err != nil {
				errs = append(errs, fmt.Errorf("parse %s: %w", filePath, err))
				return nil
			}
			configs = append(configs, *cfg)
			return nil
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("walk %s: %w", path, err))
		}
	}

	if len(configs) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("failed to load any workspace configs: %w", errors.Join(errs...))
	}

	return configs, nil
}

func loadWorkspaceConfigFile(path string) (*workspaceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var cfg workspaceConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	return &cfg, nil
}

func resolveWorkspaceEnvironment(dir, override string, configs []workspaceConfig) (string, error) {
	available := workspaceEnvironmentNames(dir, configs)
	if override != "" {
		if len(available) == 0 || slices.Contains(available, override) {
			return override, nil
		}
		return "", fmt.Errorf("environment %q not found in workspace; available environments: %s", override, strings.Join(available, ", "))
	}

	// No auto-selection: explicit environment is always required in datadir mode.
	// Set it via WithEnvironment() or the QUONFIG_ENVIRONMENT env var.
	if len(available) == 0 {
		return "", fmt.Errorf("environment required for datadir mode; set WithEnvironment() or QUONFIG_ENVIRONMENT env var")
	}
	return "", fmt.Errorf("environment required for datadir mode (available: %s); set WithEnvironment() or QUONFIG_ENVIRONMENT env var", strings.Join(available, ", "))
}

func workspaceEnvironmentNames(dir string, configs []workspaceConfig) []string {
	if names := readEnvironmentNames(filepath.Join(dir, "environments.json")); len(names) > 0 {
		return names
	}

	var names []string
	seen := make(map[string]struct{})
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	for _, cfg := range configs {
		for _, env := range cfg.Environments {
			add(env.ID)
		}
	}

	slices.Sort(names)
	return names
}

func readEnvironmentNames(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var mapped map[string]string
	if err := json.Unmarshal(data, &mapped); err == nil {
		names := make([]string, 0, len(mapped))
		for _, name := range mapped {
			names = append(names, name)
		}
		slices.Sort(names)
		return names
	}

	var raw []string
	if err := json.Unmarshal(data, &raw); err == nil {
		return raw
	}

	return nil
}
