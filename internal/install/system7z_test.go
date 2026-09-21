package install

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
)

func TestResolveSystem7zPathUsesConfiguredPath(t *testing.T) {
	tmp := t.TempDir()
	exe := filepath.Join(tmp, "custom-7z.exe")
	if err := os.WriteFile(exe, []byte("fake"), 0o755); err != nil {
		t.Fatalf("write fake 7z: %v", err)
	}

	got := resolveSystem7zPath(exe)
	assert.Eq(t, exe, got)
}

func TestResolveSystem7zPathFallsBackWhenConfiguredPathMissing(t *testing.T) {
	t.Setenv("PATH", "")

	got := resolveSystem7zPath(filepath.Join(t.TempDir(), "missing-7z.exe"))
	assert.Eq(t, "", got)
}

func TestShouldUseSystem7zForPreferredFormats(t *testing.T) {
	cases := []struct {
		name           string
		filename       string
		extractArchive bool
		want           bool
	}{
		{name: "7z", filename: "tool.7z", want: true},
		{name: "rar", filename: "tool.rar", want: true},
		{name: "msi", filename: "setup.msi", want: true},
		{name: "cab", filename: "driver.cab", want: true},
		{name: "iso", filename: "image.iso", want: true},
		{name: "exe extract archive", filename: "setup.exe", extractArchive: true, want: true},
		{name: "exe single", filename: "setup.exe", want: false},
		{name: "zip stays go", filename: "tool.zip", want: false},
		{name: "tar gz stays go", filename: "tool.tar.gz", want: false},
		{name: "tgz stays go", filename: "tool.tgz", want: false},
		{name: "tar xz stays go", filename: "tool.tar.xz", want: false},
		{name: "txz stays go", filename: "tool.txz", want: false},
		{name: "tar bz2 stays go", filename: "tool.tar.bz2", want: false},
		{name: "tbz stays go", filename: "tool.tbz", want: false},
		{name: "tar zst stays go", filename: "tool.tar.zst", want: false},
		{name: "single gz stays go", filename: "tool.gz", want: false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert.Eq(t, tt.want, shouldUseSystem7z(tt.filename, tt.extractArchive))
		})
	}
}

func TestParseSystem7zListOutput(t *testing.T) {
	output := `
Path = tool.7z
Type = 7z

----------
Path = bin/tool.exe
Size = 12
Packed Size = 8
Modified = 2026-05-13 10:00:00
Attributes = A
CRC = 12345678
Encrypted = -
Method = LZMA2
Block = 0

Path = docs/
Folder = +
Size = 0
Packed Size = 0
Attributes = D
`

	files, err := parseSystem7zListOutput([]byte(output))
	if err != nil {
		t.Fatalf("parse 7z list output: %v", err)
	}

	assert.Eq(t, 2, len(files))
	assert.Eq(t, "bin"+string(os.PathSeparator)+"tool.exe", files[0].Name)
	assert.False(t, files[0].Dir())
	assert.Eq(t, "docs", files[1].Name)
	assert.True(t, files[1].Dir())
}

func TestParseSystem7zListOutputRejectsUnsafePath(t *testing.T) {
	output := `
Path = evil.7z
Type = 7z

----------
Path = ../evil.exe
Size = 1
`

	_, err := parseSystem7zListOutput([]byte(output))
	if err == nil {
		t.Fatal("expected unsafe archive path error")
	}
	assert.Contains(t, err.Error(), "unsafe archive path")
}

func TestSystem7zExtractorSelectsCandidateAndExtractsFile(t *testing.T) {
	tmp := t.TempDir()
	var extractArgs []string
	origRunner := runSystem7zCommand
	defer func() { runSystem7zCommand = origRunner }()

	runSystem7zCommand = func(exe string, args ...string) ([]byte, error) {
		if args[0] == "l" {
			return []byte(`
Path = tool.7z
Type = 7z

----------
Path = bin/tool.exe
Size = 4
`), nil
		}
		extractArgs = append([]string(nil), args...)
		outDir := ""
		member := args[len(args)-1]
		for _, arg := range args {
			if strings.HasPrefix(arg, "-o") {
				outDir = strings.TrimPrefix(arg, "-o")
			}
		}
		if outDir == "" {
			t.Fatal("expected output dir")
		}
		extracted := filepath.Join(outDir, filepath.FromSlash(member))
		if err := os.MkdirAll(filepath.Dir(extracted), 0o755); err != nil {
			return nil, err
		}
		return nil, os.WriteFile(extracted, []byte("tool"), 0o755)
	}

	extractor := NewSystem7zExtractor("tool.7z", "tool", NewBinaryChooser("tool"), "7z")
	data := []byte("archive")
	file, candidates, err := extractor.Extract(bytes.NewReader(data), int64(len(data)), false)
	if err != nil {
		t.Fatalf("extract candidate: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected direct selected file, got candidates %#v", candidates)
	}

	out := filepath.Join(tmp, "tool.exe")
	if err := file.Extract(out); err != nil {
		t.Fatalf("extract selected file: %v", err)
	}
	data, err = os.ReadFile(out)
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	assert.Eq(t, "tool", string(data))
	assert.Contains(t, strings.Join(extractArgs, " "), "bin/tool.exe")
}

func TestSystem7zExtractorExtractAllRunsSevenZipOnce(t *testing.T) {
	tmp := t.TempDir()
	extractCalls := 0
	origRunner := runSystem7zCommand
	defer func() { runSystem7zCommand = origRunner }()

	runSystem7zCommand = func(exe string, args ...string) ([]byte, error) {
		if args[0] == "l" {
			return []byte(`
Path = setup.exe
Type = NSIS

----------
Path = $PLUGINSDIR/modern-wizard.bmp
Size = 4

Path = $PLUGINSDIR/nsDialogs.dll
Size = 3
`), nil
		}
		extractCalls++
		outDir := ""
		for _, arg := range args {
			if strings.HasPrefix(arg, "-o") {
				outDir = strings.TrimPrefix(arg, "-o")
			}
		}
		if outDir == "" {
			t.Fatal("expected output dir")
		}
		files := map[string]string{
			"$PLUGINSDIR/modern-wizard.bmp": "bmp",
			"$PLUGINSDIR/nsDialogs.dll":     "dll",
		}
		for name, content := range files {
			extracted := filepath.Join(outDir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(extracted), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(extracted, []byte(content), 0o644); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}

	chooser, err := NewFileChooser("*")
	if err != nil {
		t.Fatalf("new chooser: %v", err)
	}
	extractor := NewSystem7zExtractor("setup.exe", "setup", chooser, "7z")
	data := []byte("archive")
	_, files, err := extractor.Extract(bytes.NewReader(data), int64(len(data)), true)
	if err != nil && len(files) == 0 {
		t.Fatalf("extract candidates: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %#v", files)
	}

	for _, file := range files {
		out := filepath.Join(tmp, file.Name)
		if err := file.Extract(out); err != nil {
			t.Fatalf("extract %s: %v", file.Name, err)
		}
	}

	assert.Eq(t, 1, extractCalls)
}

func TestSystem7zExtractorExtractAllToSkipsListAndRunsSevenZipOnce(t *testing.T) {
	tmp := t.TempDir()
	extractCalls := 0
	origRunner := runSystem7zCommand
	defer func() { runSystem7zCommand = origRunner }()

	runSystem7zCommand = func(exe string, args ...string) ([]byte, error) {
		if args[0] == "l" {
			t.Fatal("did not expect 7z list for direct extract-all")
		}
		extractCalls++
		outDir := ""
		for _, arg := range args {
			if strings.HasPrefix(arg, "-o") {
				outDir = strings.TrimPrefix(arg, "-o")
			}
		}
		if outDir == "" {
			t.Fatal("expected output dir")
		}
		files := map[string]string{
			"$PLUGINSDIR/modern-wizard.bmp": "bmp",
			"$PLUGINSDIR/nsDialogs.dll":     "dll",
		}
		for name, content := range files {
			extracted := filepath.Join(outDir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(extracted), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(extracted, []byte(content), 0o644); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}

	chooser, err := NewFileChooser("*")
	if err != nil {
		t.Fatalf("new chooser: %v", err)
	}
	extractor := NewSystem7zExtractor("setup.exe", "setup", chooser, "7z")
	data := []byte("archive")
	files, err := extractor.ExtractAllTo(bytes.NewReader(data), int64(len(data)), tmp)
	if err != nil {
		t.Fatalf("extract all: %v", err)
	}

	assert.Eq(t, 2, len(files))
	assert.Eq(t, 1, extractCalls)
	data, err = os.ReadFile(filepath.Join(tmp, "$PLUGINSDIR", "modern-wizard.bmp"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	assert.Eq(t, "bmp", string(data))
}

func TestSystem7zExtractorExtractAllToWithOptionsStripsComponents(t *testing.T) {
	tmp := t.TempDir()
	origRunner := runSystem7zCommand
	defer func() { runSystem7zCommand = origRunner }()

	runSystem7zCommand = func(exe string, args ...string) ([]byte, error) {
		outDir := ""
		for _, arg := range args {
			if strings.HasPrefix(arg, "-o") {
				outDir = strings.TrimPrefix(arg, "-o")
			}
		}
		if outDir == "" {
			t.Fatal("expected output dir")
		}
		files := map[string]string{
			"ventoy-1.1.12/Ventoy2Disk.exe": "exe",
			"ventoy-1.1.12/boot/boot.img":   "boot",
		}
		for name, content := range files {
			extracted := filepath.Join(outDir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(extracted), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(extracted, []byte(content), 0o644); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}

	chooser, err := NewFileChooser("*")
	if err != nil {
		t.Fatalf("new chooser: %v", err)
	}
	extractor := NewSystem7zExtractor("ventoy.zip", "ventoy", chooser, "7z")
	data := []byte("archive")
	files, err := extractor.ExtractAllToWithOptions(bytes.NewReader(data), int64(len(data)), tmp, ArchiveExtractOptions{StripComponents: 1})
	if err != nil {
		t.Fatalf("extract all with strip: %v", err)
	}

	assert.Eq(t, 2, len(files))
	data, err = os.ReadFile(filepath.Join(tmp, "Ventoy2Disk.exe"))
	if err != nil {
		t.Fatalf("read stripped file: %v", err)
	}
	assert.Eq(t, "exe", string(data))
	if _, err := os.Stat(filepath.Join(tmp, "ventoy-1.1.12")); !os.IsNotExist(err) {
		t.Fatalf("expected stripped root directory to be absent, stat err=%v", err)
	}
}

func TestSystem7zExtractorExtractAllToFiltersChooserMatches(t *testing.T) {
	tmp := t.TempDir()
	origRunner := runSystem7zCommand
	defer func() { runSystem7zCommand = origRunner }()

	runSystem7zCommand = func(exe string, args ...string) ([]byte, error) {
		outDir := ""
		for _, arg := range args {
			if strings.HasPrefix(arg, "-o") {
				outDir = strings.TrimPrefix(arg, "-o")
			}
		}
		files := map[string]string{
			"ffmpeg/bin/ffmpeg.exe": "ffmpeg",
			"ffmpeg/doc/readme.txt": "readme",
		}
		for name, content := range files {
			extracted := filepath.Join(outDir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(extracted), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(extracted, []byte(content), 0o644); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}

	chooser, err := NewFileChooser("*.exe")
	if err != nil {
		t.Fatalf("new chooser: %v", err)
	}
	extractor := NewSystem7zExtractor("ffmpeg.7z", "ffmpeg", chooser, "7z")
	data := []byte("archive")
	files, err := extractor.ExtractAllToWithOptions(bytes.NewReader(data), int64(len(data)), tmp, ArchiveExtractOptions{StripComponents: 2})
	if err != nil {
		t.Fatalf("extract all with chooser: %v", err)
	}

	assert.Eq(t, 1, len(files))
	data, err = os.ReadFile(filepath.Join(tmp, "ffmpeg.exe"))
	if err != nil {
		t.Fatalf("read filtered exe: %v", err)
	}
	assert.Eq(t, "ffmpeg", string(data))
	if _, err := os.Stat(filepath.Join(tmp, "readme.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected non-matching file to be absent, stat err=%v", err)
	}
}

func TestCopyExtractedPathPreservesDirectoryTimestampAfterChildren(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dirTime := time.Date(2024, 6, 7, 8, 9, 10, 0, time.UTC)
	fileTime := time.Date(2024, 7, 8, 9, 10, 12, 0, time.UTC)
	file := filepath.Join(src, "resource.txt")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.WriteFile(file, []byte("resource"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	if err := os.Chtimes(file, fileTime, fileTime); err != nil {
		t.Fatalf("set source file time: %v", err)
	}
	if err := os.Chtimes(src, dirTime, dirTime); err != nil {
		t.Fatalf("set source dir time: %v", err)
	}

	dst := filepath.Join(root, "dst")
	if err := copyExtractedPath(src, dst, 0o755); err != nil {
		t.Fatalf("copy extracted path: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat copied dir: %v", err)
	}
	if !info.ModTime().Equal(dirTime) {
		t.Fatalf("expected copied dir mtime %s, got %s", dirTime, info.ModTime())
	}
}
