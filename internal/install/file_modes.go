package install

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func writeReaderWithModTime(r io.Reader, rename string, mode fs.FileMode, modTime time.Time) error {
	if rename[0] == '-' {
		_, err := io.Copy(os.Stdout, r)
		return err
	}
	os.Remove(rename)
	if err := os.MkdirAll(filepath.Dir(rename), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(rename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err = io.Copy(f, r); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return applyModTime(rename, modTime)
}

func applyModTime(path string, modTime time.Time) error {
	if modTime.IsZero() {
		return nil
	}
	return os.Chtimes(path, modTime, modTime)
}

func modeFrom(fname string, mode fs.FileMode) fs.FileMode {
	if isExec(fname, mode) {
		return mode | 0o111
	}
	return mode
}

func rename(file string, nameguess string) string {
	if isDefinitelyNotExec(file) {
		return file
	}
	switch {
	case strings.HasSuffix(file, ".appimage"):
		return file[:len(file)-len(".appimage")]
	case strings.HasSuffix(file, ".exe"):
		return file
	default:
		return nameguess
	}
}

func isDefinitelyNotExec(file string) bool {
	base := strings.ToLower(filepath.Base(file))
	switch base {
	case "license", "copying", "notice", "readme", "changelog", "changes", "authors", "contributors":
		return true
	}
	return strings.HasSuffix(file, ".deb") || strings.HasSuffix(file, ".1") || strings.HasSuffix(file, ".txt")
}

func isExec(file string, mode os.FileMode) bool {
	if isDefinitelyNotExec(file) {
		return false
	}
	return strings.HasSuffix(file, ".exe") || strings.HasSuffix(file, ".appimage") || !strings.Contains(file, ".") || mode&0o111 != 0
}
