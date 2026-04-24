package configutil

import (
	"os"
	"path/filepath"

	gconfig "github.com/gookit/config/v2"
	gtoml "github.com/gookit/config/v2/toml"
)

const FormatTOML = "toml"

func NewTOMLManager(name string) *gconfig.Config {
	cfg := gconfig.NewEmpty(name, gconfig.ParseEnv)
	cfg.AddDriver(gtoml.Driver)
	return cfg
}

func LoadTOMLFile(name, path string) (*gconfig.Config, error) {
	cfg := NewTOMLManager(name)
	if err := cfg.LoadFilesByFormat(FormatTOML, path); err != nil {
		return nil, err
	}
	return cfg, nil
}

func SaveTOMLFile(path string, cfg *gconfig.Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return cfg.DumpToFile(path, FormatTOML)
}
