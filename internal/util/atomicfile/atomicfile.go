// Package atomicfile writes whole files atomically: the payload goes into a
// temp file next to the target and is then moved over it, so an interrupted
// write can never leave a half-serialized store or config behind.
package atomicfile

import (
	"os"
	"path/filepath"
)

// WriteFile writes data to path atomically, creating parent directories.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	return WriteFunc(path, perm, func(tmp *os.File) error {
		_, err := tmp.Write(data)
		return err
	})
}

// WriteFunc lets a caller serialize into the temp file (for writers that only
// accept a path or a file), then moves it over path.
func WriteFunc(path string, perm os.FileMode, write func(tmp *os.File) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	discard := func() { _ = os.Remove(tmpPath) }

	if err := write(tmp); err != nil {
		_ = tmp.Close()
		discard()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		discard()
		return err
	}
	if err := tmp.Close(); err != nil {
		discard()
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		discard()
		return err
	}
	if err := Replace(tmpPath, path); err != nil {
		discard()
		return err
	}
	return nil
}
