package config

import (
	"os"
	"path/filepath"
)

var replaceConfigFile = platformReplaceConfigFile

// SaveAtomic serializes a config beside the target before replacing it.
func SaveAtomic(path string, file *File) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	defer os.Remove(tmpPath)

	if err := Save(tmpPath, file); err != nil {
		return err
	}
	return replaceConfigFile(tmpPath, path)
}
