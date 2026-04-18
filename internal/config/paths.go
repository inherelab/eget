package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/inherelab/eget/home"
)

type pathOptions struct {
	HomeDir   string
	GOOS      string
	LookupEnv func(string) (string, bool)
}

func ResolveConfigPath() (string, error) {
	homeDir, err := home.Home()
	if err != nil {
		return "", err
	}

	return resolveConfigPath(pathOptions{
		HomeDir:   homeDir,
		GOOS:      runtime.GOOS,
		LookupEnv: os.LookupEnv,
	})
}

func ResolveWritablePath() (string, error) {
	homeDir, err := home.Home()
	if err != nil {
		return "", err
	}
	return resolveWritablePath(pathOptions{
		HomeDir:   homeDir,
		GOOS:      runtime.GOOS,
		LookupEnv: os.LookupEnv,
	})
}

func OSConfigPath(homeDir, goos string, lookupEnv func(string) (string, bool)) string {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	return getOSConfigPath(pathOptions{
		HomeDir:   homeDir,
		GOOS:      goos,
		LookupEnv: lookupEnv,
	})
}

func resolveConfigPath(opts pathOptions) (string, error) {
	if opts.LookupEnv == nil {
		opts.LookupEnv = os.LookupEnv
	}
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}

	checkDotfile := true
	if configPath, ok := opts.LookupEnv("EGET_CONFIG"); ok && configPath != "" {
		if fileExists(configPath) {
			return configPath, nil
		}
		checkDotfile = false
	}

	if checkDotfile {
		dotfilePath := filepath.Join(opts.HomeDir, ".eget.toml")
		if fileExists(dotfilePath) {
			return dotfilePath, nil
		}
	}

	legacyPath := "eget.toml"
	if fileExists(legacyPath) {
		return legacyPath, nil
	}

	fallbackPath := getOSConfigPath(opts)
	if fileExists(fallbackPath) {
		return fallbackPath, nil
	}

	return "", os.ErrNotExist
}

func resolveWritablePath(opts pathOptions) (string, error) {
	if opts.LookupEnv == nil {
		opts.LookupEnv = os.LookupEnv
	}
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}

	if configPath, ok := opts.LookupEnv("EGET_CONFIG"); ok && configPath != "" {
		return configPath, nil
	}

	if path, err := resolveConfigPath(opts); err == nil {
		return path, nil
	}

	return filepath.Join(opts.HomeDir, ".eget.toml"), nil
}

func getOSConfigPath(opts pathOptions) string {
	switch opts.GOOS {
	case "windows":
		if dir, ok := opts.LookupEnv("LOCALAPPDATA"); ok && dir != "" {
			return filepath.Join(dir, "eget", "eget.toml")
		}
		return filepath.Join(opts.HomeDir, "LocalAppData", "eget", "eget.toml")
	default:
		if dir, ok := opts.LookupEnv("XDG_CONFIG_HOME"); ok && dir != "" {
			return filepath.Join(dir, "eget", "eget.toml")
		}
		return filepath.Join(opts.HomeDir, ".config", "eget", "eget.toml")
	}
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

func IsNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
