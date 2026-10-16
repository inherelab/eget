package install

import (
	"io"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestAutoSelectExtractedFileByArch(t *testing.T) {
	candidates := []ExtractedFile{
		{ArchiveName: `arm64\WinDirStat.exe`, Name: `arm64\WinDirStat.exe`, mode: 0o666},
		{ArchiveName: `x86\WinDirStat.exe`, Name: `x86\WinDirStat.exe`, mode: 0o666},
		{ArchiveName: `x64\WinDirStat.exe`, Name: `x64\WinDirStat.exe`, mode: 0o666},
	}

	selected, ok := autoSelectExtractedFile(candidates, "windows", "amd64")
	if !ok {
		t.Fatal("expected auto selection for amd64 candidates")
	}
	if selected.ArchiveName != `x64\WinDirStat.exe` {
		t.Fatalf("expected x64 executable to be selected, got %q", selected.ArchiveName)
	}
}

func TestAutoSelectExtractedFilePicksOnlyWindowsExe(t *testing.T) {
	candidates := []ExtractedFile{
		{ArchiveName: `LICENSE`, Name: `LICENSE`, mode: 0o666},
		{ArchiveName: `gsa.exe`, Name: `gsa.exe`, mode: 0o666},
	}

	selected, ok := autoSelectExtractedFile(candidates, "windows", "amd64")
	if !ok {
		t.Fatal("expected auto selection for the only Windows executable")
	}
	if selected.ArchiveName != `gsa.exe` {
		t.Fatalf("expected gsa.exe to be selected, got %q", selected.ArchiveName)
	}
}

func TestAutoSelectExtractedFileKeepsPromptForMultipleWindowsExe(t *testing.T) {
	candidates := []ExtractedFile{
		{ArchiveName: `gsa.exe`, Name: `gsa.exe`, mode: 0o666},
		{ArchiveName: `gsa-helper.exe`, Name: `gsa-helper.exe`, mode: 0o666},
	}

	_, ok := autoSelectExtractedFile(candidates, "windows", "amd64")
	if ok {
		t.Fatal("expected multiple Windows executables to keep prompt fallback")
	}
}

func TestResolveExtractedFileUsesExplicitSystemForWindowsExeSelection(t *testing.T) {
	runner := &InstallRunner{Stderr: io.Discard}
	candidates := []ExtractedFile{
		{ArchiveName: `LICENSE`, Name: `LICENSE`, mode: 0o666},
		{ArchiveName: `gsa.exe`, Name: `gsa.exe`, mode: 0o666},
	}

	selected, all, err := runner.resolveExtractedFile(candidates, Options{System: "windows/amd64"})
	if err != nil {
		t.Fatalf("resolve extracted file: %v", err)
	}
	if all {
		t.Fatal("expected single file selection, got extract-all")
	}
	if selected.ArchiveName != `gsa.exe` {
		t.Fatalf("expected gsa.exe to be selected, got %q", selected.ArchiveName)
	}
}

func TestAutoSelectExtractedFileKeepsPromptWhenAmbiguous(t *testing.T) {
	candidates := []ExtractedFile{
		{ArchiveName: `bin\tool.exe`, Name: `bin\tool.exe`, mode: 0o666},
		{ArchiveName: `tools\tool.exe`, Name: `tools\tool.exe`, mode: 0o666},
	}

	_, ok := autoSelectExtractedFile(candidates, "windows", "amd64")
	if ok {
		t.Fatal("expected ambiguous candidates to keep prompt fallback")
	}
}

func TestAutoExtractCurrentPlatformExecutablesFiltersOtherPlatformExecutables(t *testing.T) {
	candidates := []ExtractedFile{
		{ArchiveName: `x86_64\uv.exe`, Name: `x86_64\uv.exe`, mode: 0o666},
		{ArchiveName: `x86\uv.exe`, Name: `x86\uv.exe`, mode: 0o666},
		{ArchiveName: `linux\uv`, Name: `linux\uv`, mode: 0o755},
	}

	selected, ok := autoExtractCurrentPlatformExecutables(candidates, Options{System: "windows/amd64"})
	if !ok {
		t.Fatal("expected current-platform executable selection")
	}
	assert.Eq(t, 1, len(selected))
	assert.Eq(t, `x86_64\uv.exe`, selected[0].ArchiveName)
}

func TestStripSharedPackageDir(t *testing.T) {
	t.Run("strip shared package dir", func(t *testing.T) {
		files := []ExtractedFile{
			{ArchiveName: `uv-x86_64-unknown-linux-gnu/uv`, Name: `uv-x86_64-unknown-linux-gnu/uv`, mode: 0o755},
			{ArchiveName: `uv-x86_64-unknown-linux-gnu/uvx`, Name: `uv-x86_64-unknown-linux-gnu/uvx`, mode: 0o755},
		}
		got := stripSharedPackageDir(files)
		assert.Eq(t, []string{"uv", "uvx"}, []string{got[0].Name, got[1].Name})
		assert.Eq(t, "uv-x86_64-unknown-linux-gnu/uv", got[0].ArchiveName)
	})

	t.Run("keep real sub-dir structure", func(t *testing.T) {
		files := []ExtractedFile{
			{ArchiveName: `herdr.exe`, Name: `herdr.exe`, mode: 0o666},
			{ArchiveName: `conpty/x64/OpenConsole.exe`, Name: `conpty/x64/OpenConsole.exe`, mode: 0o666},
		}
		got := stripSharedPackageDir(files)
		assert.Eq(t, []string{"herdr.exe", "conpty/x64/OpenConsole.exe"}, []string{got[0].Name, got[1].Name})
	})

	t.Run("keep structure on multi top dirs", func(t *testing.T) {
		files := []ExtractedFile{
			{ArchiveName: `bin/a.exe`, Name: `bin/a.exe`, mode: 0o666},
			{ArchiveName: `tools/b.exe`, Name: `tools/b.exe`, mode: 0o666},
		}
		got := stripSharedPackageDir(files)
		assert.Eq(t, []string{"bin/a.exe", "tools/b.exe"}, []string{got[0].Name, got[1].Name})
	})
}
