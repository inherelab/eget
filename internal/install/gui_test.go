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

func TestDefaultInstallerLauncherCommand(t *testing.T) {
	launcher := DefaultInstallerLauncher{GOOS: "windows"}
	cmd, args, err := launcher.command("C:/Temp/app.msi", InstallerKindMSI)
	if err != nil {
		t.Fatalf("msi command: %v", err)
	}
	if cmd != "powershell" || len(args) != 6 || args[0] != "-NoProfile" || args[4] != "Start-Process -FilePath 'msiexec.exe' -ArgumentList @('/i', $args[0]) -Verb RunAs" || args[5] != "C:/Temp/app.msi" {
		t.Fatalf("unexpected msi command: %s %#v", cmd, args)
	}
	cmd, args, err = launcher.command("C:/Temp/setup.exe", InstallerKindEXE)
	if err != nil {
		t.Fatalf("exe command: %v", err)
	}
	if cmd != "powershell" || len(args) != 6 || args[0] != "-NoProfile" || args[4] != "Start-Process -FilePath $args[0] -Verb RunAs" || args[5] != "C:/Temp/setup.exe" {
		t.Fatalf("unexpected exe command: %s %#v", cmd, args)
	}
}

func TestDefaultInstallerLauncherRejectsUnsupportedPlatform(t *testing.T) {
	goos := runtime.GOOS
	if goos == "windows" {
		goos = "linux"
	}
	launcher := DefaultInstallerLauncher{GOOS: goos}
	if _, _, err := launcher.command("/tmp/app.msi", InstallerKindMSI); err == nil {
		t.Fatal("expected non-windows msi launcher to fail")
	}
}
