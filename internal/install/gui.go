package install

import (
	"fmt"
	"os/exec"
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
	cmdName, args, err := l.command(path, kind)
	if err != nil {
		return err
	}
	return exec.Command(cmdName, args...).Start()
}

func (l DefaultInstallerLauncher) command(path string, kind InstallerKind) (string, []string, error) {
	goos := l.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos != "windows" {
		return "", nil, fmt.Errorf("launching GUI installer %s is unsupported on %s", filepath.Base(path), goos)
	}
	switch kind {
	case InstallerKindMSI:
		return "powershell", []string{
			"-NoProfile",
			"-ExecutionPolicy", "Bypass",
			"-Command", "Start-Process -FilePath 'msiexec.exe' -ArgumentList @('/i', $args[0]) -Verb RunAs",
			path,
		}, nil
	case InstallerKindEXE:
		return "powershell", []string{
			"-NoProfile",
			"-ExecutionPolicy", "Bypass",
			"-Command", "Start-Process -FilePath $args[0] -Verb RunAs",
			path,
		}, nil
	default:
		return "", nil, fmt.Errorf("unsupported installer kind for %s", filepath.Base(path))
	}
}
