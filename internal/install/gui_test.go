package install

import (
	"runtime"
	"testing"
)

func TestDetectGUIInstallMode(t *testing.T) {
	tests := []struct {
		name  string
		isGUI bool
		file  string
		want  string
	}{
		{"non gui msi stays empty", false, "app.msi", ""},
		{"gui msi installer", true, "app.msi", InstallModeInstaller},
		{"gui setup exe installer", true, "PicoClaw-Setup.exe", InstallModeInstaller},
		{"gui installer exe installer", true, "foo-installer-x64.exe", InstallModeInstaller},
		{"gui plain exe portable", true, "picoclaw.exe", InstallModePortable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectGUIInstallMode(tt.isGUI, tt.file)
			if got != tt.want {
				t.Fatalf("DetectGUIInstallMode(%t, %q) = %q, want %q", tt.isGUI, tt.file, got, tt.want)
			}
		})
	}
}

func TestWindowsInstallerCommand(t *testing.T) {
	file, args, err := windowsInstallerCommand("C:/Temp/app.msi", InstallerKindMSI)
	if err != nil {
		t.Fatalf("msi command: %v", err)
	}
	if file != "msiexec.exe" || args != `/i "C:/Temp/app.msi"` {
		t.Fatalf("unexpected msi command: file=%s args=%s", file, args)
	}
	file, args, err = windowsInstallerCommand("C:/Temp/setup.exe", InstallerKindEXE)
	if err != nil {
		t.Fatalf("exe command: %v", err)
	}
	if file != "C:/Temp/setup.exe" || args != "" {
		t.Fatalf("unexpected exe command: file=%s args=%s", file, args)
	}
}

func TestDefaultInstallerLauncherRejectsUnsupportedPlatform(t *testing.T) {
	goos := runtime.GOOS
	if goos == "windows" {
		goos = "linux"
	}
	launcher := DefaultInstallerLauncher{GOOS: goos}
	if err := launcher.LaunchInstaller("/tmp/app.msi", InstallerKindMSI); err == nil {
		t.Fatal("expected non-windows msi launcher to fail")
	}
}
