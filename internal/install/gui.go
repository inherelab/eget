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

type InstallerLauncher interface {
	LaunchInstaller(path string, kind InstallerKind) error
}

type DefaultInstallerLauncher struct {
	GOOS string
}

func DetectGUIInstallMode(isGUI bool, fileName string) string {
	if !isGUI {
		return ""
	}
	if DetectInstallerKind(fileName) != InstallerKindUnknown {
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

func (l DefaultInstallerLauncher) LaunchInstaller(path string, kind InstallerKind) error {
	goos := l.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos != "windows" {
		return fmt.Errorf("launching GUI installer %s is unsupported on %s", filepath.Base(path), goos)
	}
	return launchWindowsInstaller(path, kind)
}

func windowsInstallerCommand(path string, kind InstallerKind) (string, string, error) {
	switch kind {
	case InstallerKindMSI:
		return "msiexec.exe", fmt.Sprintf("/i %q", path), nil
	case InstallerKindEXE:
		return path, "", nil
	default:
		return "", "", fmt.Errorf("unsupported installer kind for %s", filepath.Base(path))
	}
}
