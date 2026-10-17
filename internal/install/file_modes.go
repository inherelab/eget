package install

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/inherelab/eget/internal/filekind"
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
	f, err := openReplaceFile(rename, mode)
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
	switch filekind.ExecutableSuffixExact(file) {
	case filekind.ExtAppImage:
		return file[:len(file)-len(filekind.ExtAppImage)]
	case filekind.ExtExe:
		return file
	default:
		return nameguess
	}
}

// notExecBaseNames are base names that never denote an executable.
var notExecBaseNames = map[string]bool{
	"license": true, "copying": true, "notice": true, "readme": true,
	"changelog": true, "changes": true, "authors": true, "contributors": true,
}

func isDefinitelyNotExec(file string) bool {
	if notExecBaseNames[strings.ToLower(filepath.Base(file))] {
		return true
	}
	return filekind.IsNonExecutableExact(file)
}

func isExec(file string, mode os.FileMode) bool {
	if isDefinitelyNotExec(file) {
		return false
	}
	return filekind.IsExecutableExact(file) || !strings.Contains(file, ".") || mode&0o111 != 0
}
