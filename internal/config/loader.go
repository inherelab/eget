package config

import (
	"os"
)

func Load() (*File, error) {
	path, err := ResolveConfigPath()
	if err != nil {
		if IsNotExist(err) {
			return NewFile(), nil
		}
		return nil, err
	}

	file, err := LoadFile(path)
	if err != nil {
		if IsNotExist(err) {
			return NewFile(), nil
		}
		return nil, err
	}

	return file, nil
}

func LoadFile(path string) (*File, error) {
	cfg, err := loadConfigManager(path)
	if err != nil {
		return nil, err
	}
	return decodeConfigFile(cfg)
}

func NewFile() *File {
	cfg := &File{}
	cfg.Repos = make(map[string]Section)
	cfg.Packages = make(map[string]Section)
	return cfg
}

func LoadFromEnvOrDefault(lookupEnv func(string) (string, bool), homeDir, goos string) (*File, string, error) {
	path, err := resolveConfigPath(pathOptions{
		HomeDir:   homeDir,
		GOOS:      goos,
		LookupEnv: lookupEnv,
	})
	if err != nil {
		if os.IsNotExist(err) {
			return NewFile(), "", nil
		}
		return nil, "", err
	}

	cfg, err := LoadFile(path)
	if err != nil {
		return nil, "", err
	}

	return cfg, path, nil
}
