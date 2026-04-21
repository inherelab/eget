package config

import (
	"os"

	"github.com/BurntSushi/toml"
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
	conf := NewFile()

	var decoded mergedFile
	meta, err := toml.DecodeFile(path, &decoded)
	if err != nil {
		return nil, err
	}

	conf.Global = decoded.Global
	conf.ApiCache = decoded.ApiCache
	conf.Ghproxy = decoded.Ghproxy
	if decoded.Packages != nil {
		conf.Packages = decoded.Packages
	}

	repos := make(map[string]Section)
	meta, err = toml.DecodeFile(path, &repos)
	if err != nil {
		return nil, err
	}

	delete(repos, "global")
	delete(repos, "api_cache")
	delete(repos, "ghproxy")
	delete(repos, "packages")

	conf.Repos = repos
	conf.Meta.Keys = make([]string, len(meta.Keys()))
	for i, key := range meta.Keys() {
		conf.Meta.Keys[i] = key.String()
	}
	conf.Meta.MetaData = &meta

	return conf, nil
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
