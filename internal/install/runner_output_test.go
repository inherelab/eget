package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestOutputPathUsesHeuristicExecutableRename(t *testing.T) {
	file := ExtractedFile{Name: "chlog-windows-amd64.exe", mode: 0o666}
	got, err := outputPath(file, "", false, "", false)
	if err != nil {
		t.Fatalf("outputPath(): %v", err)
	}
	if got != "chlog.exe" {
		t.Fatalf("expected heuristic output name chlog.exe, got %q", got)
	}
}

func TestOutputPathUsesPreferredNameForExecutable(t *testing.T) {
	file := ExtractedFile{Name: "chlog-windows-amd64.exe", mode: 0o666}
	got, err := outputPath(file, "", false, "chlog", false)
	if err != nil {
		t.Fatalf("outputPath(): %v", err)
	}
	if got != "chlog.exe" {
		t.Fatalf("expected preferred output name chlog.exe, got %q", got)
	}
}

func TestOutputPathUsesPreferredNameForVersionedPortableExecutable(t *testing.T) {
	file := ExtractedFile{Name: "Alacritty-v0.17.0-portable.exe", mode: 0o666}
	got, err := outputPath(file, "", false, "alacritty", false)
	if err != nil {
		t.Fatalf("outputPath(): %v", err)
	}
	if got != "alacritty.exe" {
		t.Fatalf("expected preferred output name alacritty.exe, got %q", got)
	}
}

func TestOutputPathKeepsExecutableNameWhenPreferredNameDoesNotMatchPlatformSuffix(t *testing.T) {
	file := ExtractedFile{Name: "bd.exe", mode: 0o666}
	got, err := outputPath(file, "", false, "beads", false)
	if err != nil {
		t.Fatalf("outputPath(): %v", err)
	}
	if got != "bd.exe" {
		t.Fatalf("expected executable name bd.exe to be preserved, got %q", got)
	}
}

func TestOutputPathUsesPreferredNameWithExplicitExtension(t *testing.T) {
	file := ExtractedFile{Name: "chlog-windows-amd64.exe", mode: 0o666}
	got, err := outputPath(file, "", false, "custom-name.exe", false)
	if err != nil {
		t.Fatalf("outputPath(): %v", err)
	}
	if got != "custom-name.exe" {
		t.Fatalf("expected preferred explicit output name custom-name.exe, got %q", got)
	}
}

func TestOutputPathKeepsExplicitFileOutput(t *testing.T) {
	file := ExtractedFile{Name: "chlog-windows-amd64.exe", mode: 0o666}
	outputFile := filepath.Join(t.TempDir(), "custom-tool")
	got, err := outputPath(file, outputFile, false, "", true)
	if err != nil {
		t.Fatalf("outputPath(): %v", err)
	}
	if got != outputFile {
		t.Fatalf("expected explicit file output %q, got %q", outputFile, got)
	}
}

func TestOutputPathTreatsExplicitTrailingSeparatorAsDirectory(t *testing.T) {
	file := ExtractedFile{Name: "default-arm64-v8a.jar", mode: 0o666}
	outputDir := filepath.Join(t.TempDir(), "v1.1.7") + string(os.PathSeparator)
	got, err := outputPath(file, outputDir, false, "", true)
	if err != nil {
		t.Fatalf("outputPath(): %v", err)
	}
	assert.Eq(t, filepath.Join(outputDir, "default-arm64-v8a.jar"), got)
}

func TestOutputPathKeepsArchiveDirectoriesForExtractAll(t *testing.T) {
	file := ExtractedFile{Name: "Far/7-ZipEng.hlf", mode: 0o644}
	got, err := outputPath(file, "dist", true, "", false)
	if err != nil {
		t.Fatalf("outputPath(): %v", err)
	}
	want := filepath.Join("dist", "Far", "7-ZipEng.hlf")
	if got != want {
		t.Fatalf("expected extract-all output path %q, got %q", want, got)
	}
}

func TestOutputPathAppliesRenameFileForExtractAll(t *testing.T) {
	file := ExtractedFile{Name: "codex-x86_64-pc-windows-msvc.exe", mode: 0o666}
	got, err := outputPath(file, "bin", true, "", false, map[string]string{
		"codex-x86_64-pc-windows-msvc.exe": "codex.exe",
	})
	if err != nil {
		t.Fatalf("outputPath(): %v", err)
	}
	want := filepath.Join("bin", "codex.exe")
	if got != want {
		t.Fatalf("expected renamed extract-all output path %q, got %q", want, got)
	}
}

func TestOutputPathMatchesRenameFileWithSlashArchivePath(t *testing.T) {
	file := ExtractedFile{
		Name:        `UX\WindowSpy.ahk`,
		ArchiveName: `UX\WindowSpy.ahk`,
		mode:        0o666,
	}
	got, err := outputPath(file, "bin", true, "", false, map[string]string{
		"UX/WindowSpy.ahk": "WindowSpy.ahk",
	})

	assert.NoErr(t, err)
	assert.Eq(t, filepath.Join("bin", "WindowSpy.ahk"), got)
}

func TestOutputPathRejectsUnsafeRenameFileTarget(t *testing.T) {
	file := ExtractedFile{Name: "codex.exe", mode: 0o666}
	_, err := outputPath(file, "bin", true, "", false, map[string]string{
		"codex.exe": "../codex.exe",
	})
	if err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("expected unsafe archive path error, got %v", err)
	}
}

func TestOutputPathRejectsArchivePathTraversalForExtractAll(t *testing.T) {
	file := ExtractedFile{Name: "../evil.exe", mode: 0o644}
	if _, err := outputPath(file, "dist", true, "", false); err == nil {
		t.Fatal("expected archive path traversal to be rejected")
	}
}
