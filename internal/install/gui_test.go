package install

import (
	"runtime"
	"strings"
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
		{"gui plain exe installer", true, "picoclaw.exe", InstallModeInstaller},
		{"gui portable exe portable", true, "picoclaw-portable.exe", InstallModePortable},
		{"gui portable zip portable", true, "picoclaw-portable.zip", InstallModePortable},
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
	file, args, err := windowsInstallerCommand("C:/Temp/app.msi", InstallerKindMSI, false)
	if err != nil {
		t.Fatalf("msi command: %v", err)
	}
	if file != "msiexec.exe" || args != `/i "C:/Temp/app.msi"` {
		t.Fatalf("unexpected msi command: file=%s args=%s", file, args)
	}
	file, args, err = windowsInstallerCommand("C:/Temp/setup.exe", InstallerKindEXE, false)
	if err != nil {
		t.Fatalf("exe command: %v", err)
	}
	if file != "C:/Temp/setup.exe" || args != "" {
		t.Fatalf("unexpected exe command: file=%s args=%s", file, args)
	}
}

func TestWindowsInstallerCommandSilentFlags(t *testing.T) {
	_, silentArgs, err := windowsInstallerCommand(`C:\tmp\app.msi`, InstallerKindMSI, true)
	if err != nil {
		t.Fatalf("silent msi command: %v", err)
	}
	if !strings.Contains(silentArgs, "/qn") || !strings.Contains(silentArgs, "/norestart") {
		t.Fatalf("expected unattended msi flags, got %q", silentArgs)
	}

	_, plainArgs, err := windowsInstallerCommand(`C:\tmp\app.msi`, InstallerKindMSI, false)
	if err != nil {
		t.Fatalf("interactive msi command: %v", err)
	}
	if strings.Contains(plainArgs, "/qn") {
		t.Fatalf("interactive msi command must not be silent: %q", plainArgs)
	}

	// EXE installers disagree on flags, so silent leaves them untouched.
	file, exeArgs, err := windowsInstallerCommand(`C:\tmp\app-setup.exe`, InstallerKindEXE, true)
	if err != nil {
		t.Fatalf("exe command: %v", err)
	}
	if file != `C:\tmp\app-setup.exe` || exeArgs != "" {
		t.Fatalf("unexpected exe command: %q %q", file, exeArgs)
	}
}

func TestDefaultInstallerLauncherRejectsUnsupportedPlatform(t *testing.T) {
	goos := runtime.GOOS
	if goos == "windows" {
		goos = "linux"
	}
	launcher := DefaultInstallerLauncher{GOOS: goos}
	if err := launcher.LaunchInstaller("/tmp/app.msi", InstallerKindMSI, false); err == nil {
		t.Fatal("expected non-windows msi launcher to fail")
	}
}
