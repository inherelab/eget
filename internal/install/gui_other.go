//go:build !windows

package install

import (
	"fmt"
	"path/filepath"
	"runtime"
)

// launchWindowsInstaller keeps the signature of the windows implementation so
// the shared call site in LaunchInstaller compiles on every platform; silent is
// unused because no non-windows platform can launch a GUI installer.
func launchWindowsInstaller(path string, kind InstallerKind, silent bool) error {
	return fmt.Errorf("launching GUI installer %s is unsupported on %s", filepath.Base(path), runtime.GOOS)
}
