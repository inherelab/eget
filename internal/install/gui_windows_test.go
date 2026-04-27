//go:build windows

package install

import "testing"

func TestLaunchWindowsInstallerUsesRunAsShellExecute(t *testing.T) {
	orig := shellExecute
	defer func() { shellExecute = orig }()

	var gotVerb, gotFile, gotArgs string
	shellExecute = func(verb, file, args string) error {
		gotVerb = verb
		gotFile = file
		gotArgs = args
		return nil
	}

	if err := launchWindowsInstaller("C:/Temp/setup.exe", InstallerKindEXE); err != nil {
		t.Fatalf("launch exe installer: %v", err)
	}
	if gotVerb != "runas" || gotFile != "C:/Temp/setup.exe" || gotArgs != "" {
		t.Fatalf("unexpected shell execute call verb=%q file=%q args=%q", gotVerb, gotFile, gotArgs)
	}

	if err := launchWindowsInstaller("C:/Temp/app.msi", InstallerKindMSI); err != nil {
		t.Fatalf("launch msi installer: %v", err)
	}
	if gotVerb != "runas" || gotFile != "msiexec.exe" || gotArgs != `/i "C:/Temp/app.msi"` {
		t.Fatalf("unexpected shell execute call verb=%q file=%q args=%q", gotVerb, gotFile, gotArgs)
	}
}
