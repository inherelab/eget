package installed

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	forge "github.com/inherelab/eget/internal/source/forge"
	"github.com/inherelab/eget/internal/source/sourceforge"
	"github.com/inherelab/eget/internal/util"
)

type Options struct {
	HomeDir   string
	GOOS      string
	LookupEnv func(string) (string, bool)
}

type Store struct {
	opts Options
}

func NewStore(opts Options) *Store {
	if opts.LookupEnv == nil {
		opts.LookupEnv = os.LookupEnv
	}
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}
	return &Store{opts: opts}
}

func DefaultStore() (*Store, error) {
	homeDir, err := util.Home()
	if err != nil {
		return nil, err
	}
	return NewStore(Options{
		HomeDir:   homeDir,
		GOOS:      runtime.GOOS,
		LookupEnv: os.LookupEnv,
	}), nil
}

func (s *Store) Path() string {
	legacyPath := filepath.Join(s.opts.HomeDir, ".eget.installed.toml")
	if fileExists(legacyPath) {
		return legacyPath
	}
	return s.fallbackPath()
}

func (s *Store) Load() (*Config, error) {
	configPath := s.Path()

	cfg, err := loadStoreConfigManager(configPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load installed config: %w", err)
	}
	if os.IsNotExist(err) {
		return &Config{Installed: make(map[string]Entry)}, nil
	}

	config, err := decodeStoreConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode installed config: %w", err)
	}

	if config.Installed == nil {
		config.Installed = make(map[string]Entry)
	}
	return config, nil
}

func (s *Store) Save(config *Config) error {
	configPath := s.Path()

	if config.Installed == nil {
		config.Installed = make(map[string]Entry)
	}

	if err := saveStoreConfig(configPath, config); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}

func (s *Store) Record(target string, entry Entry) error {
	config, err := s.Load()
	if err != nil {
		return err
	}

	key := NormalizeRepoName(target)
	entry.Repo = key
	if config.Installed == nil {
		config.Installed = make(map[string]Entry)
	}
	config.Installed[key] = entry

	return s.Save(config)
}

func (s *Store) Remove(target string) error {
	config, err := s.Load()
	if err != nil {
		return err
	}

	delete(config.Installed, NormalizeRepoName(target))
	return s.Save(config)
}

func (s *Store) fallbackPath() string {
	if dir, ok := s.opts.LookupEnv("XDG_CONFIG_HOME"); ok && dir != "" {
		return filepath.Join(dir, "eget", "installed.toml")
	}
	return filepath.Join(s.opts.HomeDir, ".config", "eget", "installed.toml")
}

func NormalizeRepoName(target string) string {
	if sfTarget, err := sourceforge.ParseTarget(target); err == nil {
		return sfTarget.Normalized
	}
	if forgeTarget, err := forge.ParseTarget(target); err == nil {
		return forgeTarget.Normalized
	}

	if strings.Contains(target, "github.com/") {
		parts := strings.Split(target, "github.com/")
		if len(parts) > 1 {
			path := parts[1]
			path = strings.TrimSuffix(path, "/")
			path = strings.TrimSuffix(path, ".git")
			pathParts := strings.Split(path, "/")
			if len(pathParts) >= 2 {
				return pathParts[0] + "/" + pathParts[1]
			}
			return path
		}
	}

	if strings.Count(target, "/") == 1 && !strings.Contains(target, "://") {
		return target
	}

	return strings.TrimSuffix(target, "/")
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}

	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
