package installed

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	cfgpkg "github.com/inherelab/eget/internal/config"
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
	if util.FileExists(legacyPath) {
		return legacyPath
	}
	return s.fallbackPath()
}

// storeMu serializes read-modify-write cycles on the installed store. The web
// console serves requests concurrently, and an unlocked load-then-save would
// lose entries.
var storeMu sync.Mutex

func (s *Store) Load() (*Config, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	return s.load()
}

func (s *Store) load() (*Config, error) {
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
	storeMu.Lock()
	defer storeMu.Unlock()
	return s.save(config)
}

func (s *Store) save(config *Config) error {
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
	storeMu.Lock()
	defer storeMu.Unlock()

	config, err := s.load()
	if err != nil {
		return err
	}

	key := NormalizeRepoName(target)
	if entry.Repo == "" {
		entry.Repo = key
	}
	if config.Installed == nil {
		config.Installed = make(map[string]Entry)
	}
	recordedAt := compactStoreTime(entry.InstalledAt)
	entry.InstalledAt = recordedAt
	legacyKey := NormalizeRepoName(entry.Repo)
	existing, hasExisting := config.Installed[key]
	if !hasExisting && legacyKey != "" && legacyKey != key {
		existing, hasExisting = config.Installed[legacyKey]
	}
	if hasExisting {
		if !existing.InstalledAt.IsZero() {
			entry.InstalledAt = compactStoreTime(existing.InstalledAt)
		}
		if entry.UpdatedAt.IsZero() {
			entry.UpdatedAt = recordedAt
			if entry.UpdatedAt.IsZero() {
				entry.UpdatedAt = compactStoreTime(time.Now())
			}
		}
	}
	entry.UpdatedAt = compactStoreTime(entry.UpdatedAt)
	config.Installed[key] = entry
	if legacyKey != "" && legacyKey != key {
		delete(config.Installed, legacyKey)
	}

	return s.save(config)
}

func compactStoreTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return value.UTC().Truncate(time.Second)
}

func (s *Store) Remove(target string) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	config, err := s.load()
	if err != nil {
		return err
	}

	delete(config.Installed, NormalizeRepoName(target))
	return s.save(config)
}

func (s *Store) fallbackPath() string {
	return filepath.Join(filepath.Dir(cfgpkg.OSConfigPath(s.opts.HomeDir, s.opts.GOOS, s.opts.LookupEnv)), "installed.toml")
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
