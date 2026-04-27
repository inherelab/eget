//go:build !windows

package install

import (
	"fmt"
	"path/filepath"
	"runtime"
)

func launchWindowsInstaller(path string, kind InstallerKind) error {
	return fmt.Errorf("launching GUI installer %s is unsupported on %s", filepath.Base(path), runtime.GOOS)
}
