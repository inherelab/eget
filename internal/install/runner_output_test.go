package install

import (
	"bytes"
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

func TestOutputPathUsesPreferredNameForRenamedSingleFileAsset(t *testing.T) {
	extractor := NewExtractor("omp-darwin-arm64", "oh-my-pi", nil)
	file, _, err := extractor.Extract(bytes.NewReader(nil), 0, false)
	assert.NoErr(t, err)
	assert.Eq(t, "oh-my-pi", file.Name)

	got, err := outputPath(file, "bin", false, "omp", false)
	if err != nil {
		t.Fatalf("outputPath(): %v", err)
	}
	assert.Eq(t, filepath.Join("bin", "omp"), got)
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

func TestOutputPathAppliesGlobRenamePatternForExtractAll(t *testing.T) {
	file := ExtractedFile{Name: "MobaXterm_Personal_26.6.exe", mode: 0o666}
	got, err := outputPath(file, "bin", true, "", false, map[string]string{
		"MobaXterm_Personal_*.exe": "MobaXterm.exe",
	})

	assert.NoErr(t, err)
	assert.Eq(t, filepath.Join("bin", "MobaXterm.exe"), got)
}

func TestOutputPathAppliesGlobRenamePatternOnArchivePath(t *testing.T) {
	file := ExtractedFile{Name: "CygUtils.plugin", ArchiveName: "plugins/CygUtils.plugin", mode: 0o666}
	got, err := outputPath(file, "bin", true, "", false, map[string]string{
		"plugins/*.plugin": "CygUtils.plugin",
	})

	assert.NoErr(t, err)
	assert.Eq(t, filepath.Join("bin", "CygUtils.plugin"), got)
}

func TestOutputPathPrefersLiteralRenameOverGlobPattern(t *testing.T) {
	file := ExtractedFile{Name: "MobaXterm_Personal_26.6.exe", mode: 0o666}
	got, err := outputPath(file, "bin", true, "", false, map[string]string{
		"*.exe":                       "any.exe",
		"MobaXterm_Personal_26.6.exe": "MobaXterm.exe",
	})

	assert.NoErr(t, err)
	assert.Eq(t, filepath.Join("bin", "MobaXterm.exe"), got)
}

func TestValidateRenamePatternsRejectsInvalidGlob(t *testing.T) {
	err := validateRenamePatterns(map[string]string{"MobaXterm_Personal_[*.exe": "MobaXterm.exe"})

	if err == nil || !strings.Contains(err.Error(), "invalid rename pattern") {
		t.Fatalf("expected invalid rename pattern error, got %v", err)
	}
	assert.NoErr(t, validateRenamePatterns(map[string]string{"MobaXterm_Personal_*.exe": "MobaXterm.exe"}))
}

func TestWarnUnmatchedRenamePatterns(t *testing.T) {
	var buf bytes.Buffer

	warnUnmatchedRenamePatterns(&buf, map[string]string{
		"MobaXterm_Personal_*.exe": "MobaXterm.exe",
		"missing-*.dll":            "missing.dll",
	}, []string{filepath.Join("bin", "MobaXterm.exe")})

	got := buf.String()
	assert.NotContains(t, got, "MobaXterm_Personal_*.exe")
	assert.Contains(t, got, "missing-*.dll")
	assert.Contains(t, got, "matched no extracted file")
}

func TestExplicitFileTarget(t *testing.T) {
	t.Run("file path", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "MobaXterm.exe")
		assert.Eq(t, target, explicitFileTarget(Options{Output: target, OutputExplicit: true}))
	})

	t.Run("existing directory", func(t *testing.T) {
		dir := t.TempDir()
		assert.Eq(t, "", explicitFileTarget(Options{Output: dir, OutputExplicit: true}))
	})

	t.Run("extension-less path is a directory", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "dist")
		assert.Eq(t, "", explicitFileTarget(Options{Output: target, OutputExplicit: true}))
	})

	t.Run("default output", func(t *testing.T) {
		assert.Eq(t, "", explicitFileTarget(Options{Output: filepath.Join(t.TempDir(), "tool")}))
	})

	t.Run("stdout", func(t *testing.T) {
		assert.Eq(t, "", explicitFileTarget(Options{Output: "-", OutputExplicit: true}))
	})
}

func TestCollectSingleFileOutputMovesFileToTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "MobaXterm.exe")
	source := filepath.Join(target, "MobaXterm_Personal_26.6.exe")
	assert.NoErr(t, os.MkdirAll(target, 0o755))
	assert.NoErr(t, os.WriteFile(source, []byte("moba"), 0o644))

	paths, err := collectSingleFileOutput(target, []string{source})

	assert.NoErr(t, err)
	assert.Eq(t, []string{target}, paths)
	info, err := os.Stat(target)
	assert.NoErr(t, err)
	assert.False(t, info.IsDir())
	body, err := os.ReadFile(target)
	assert.NoErr(t, err)
	assert.Eq(t, "moba", string(body))
	entries, err := os.ReadDir(root)
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(entries))
}

func TestCollectSingleFileOutputRejectsMultipleFiles(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "MobaXterm.exe")
	assert.NoErr(t, os.MkdirAll(target, 0o755))
	assert.NoErr(t, os.WriteFile(filepath.Join(target, "a.exe"), []byte("a"), 0o644))

	_, err := collectSingleFileOutput(target, []string{filepath.Join(target, "a.exe"), filepath.Join(target, "b.plugin")})

	if err == nil || !strings.Contains(err.Error(), "is a file target but 2 files") {
		t.Fatalf("expected file target error, got %v", err)
	}
	entries, readErr := os.ReadDir(root)
	assert.NoErr(t, readErr)
	assert.Eq(t, 0, len(entries))
}
