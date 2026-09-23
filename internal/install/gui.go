package install

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

type InstallerKind string

const (
	InstallerKindUnknown InstallerKind = ""
	InstallerKindMSI     InstallerKind = "msi"
	InstallerKindEXE     InstallerKind = "exe"
)

// InstallerLauncher starts a downloaded GUI installer. silent asks for an
// unattended install where the installer kind has an unambiguous convention.
type InstallerLauncher interface {
	LaunchInstaller(path string, kind InstallerKind, silent bool) error
}

type DefaultInstallerLauncher struct {
	GOOS string
}

func DetectGUIInstallMode(isGUI bool, fileName string) string {
	if !isGUI {
		return ""
	}
	lower := strings.ToLower(filepath.Base(fileName))
	if strings.Contains(lower, "portable") {
		return InstallModePortable
	}
	switch filepath.Ext(lower) {
	case ".exe", ".msi":
		return InstallModeInstaller
	}
	return InstallModePortable
}

func DetectInstallerKind(fileName string) InstallerKind {
	lower := strings.ToLower(filepath.Base(fileName))
	switch {
	case strings.HasSuffix(lower, ".msi"):
		return InstallerKindMSI
	case strings.HasSuffix(lower, ".exe") && (strings.Contains(lower, "setup") || strings.Contains(lower, "installer")):
		return InstallerKindEXE
	default:
		return InstallerKindUnknown
	}
}

func (l DefaultInstallerLauncher) LaunchInstaller(path string, kind InstallerKind, silent bool) error {
	goos := l.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos != "windows" {
		return fmt.Errorf("launching GUI installer %s is unsupported on %s", filepath.Base(path), goos)
	}
	return launchWindowsInstaller(path, kind, silent)
}

// windowsInstallerCommand renders the launcher target and arguments. Silent
// flags are only added for MSI, where `/qn` is a firm convention; EXE
// installers (NSIS, Inno Setup, custom bootstrappers) disagree on their flags,
// so those are left to the package's install_args.
func windowsInstallerCommand(path string, kind InstallerKind, silent bool) (string, string, error) {
	switch kind {
	case InstallerKindMSI:
		if silent {
			return "msiexec.exe", fmt.Sprintf("/i %q /qn /norestart", path), nil
		}
		return "msiexec.exe", fmt.Sprintf("/i %q", path), nil
	case InstallerKindEXE:
		return path, "", nil
	default:
		return "", "", fmt.Errorf("unsupported installer kind for %s", filepath.Base(path))
	}
}
