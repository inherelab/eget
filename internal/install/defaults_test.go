package install

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewFileChooserSupportsCommaSeparatedPatterns(t *testing.T) {
	chooser, err := NewFileChooser("README*, LICENSE")
	if err != nil {
		t.Fatalf("NewFileChooser(): %v", err)
	}

	if direct, possible := chooser.Choose("README.md", false, 0); direct || !possible {
		t.Fatalf("expected README.md to match, got direct=%t possible=%t", direct, possible)
	}
	if direct, possible := chooser.Choose("docs/LICENSE", false, 0); direct || !possible {
		t.Fatalf("expected docs/LICENSE to match, got direct=%t possible=%t", direct, possible)
	}
	if direct, possible := chooser.Choose("bin/tool.exe", false, 0); direct || possible {
		t.Fatalf("expected bin/tool.exe to be ignored, got direct=%t possible=%t", direct, possible)
	}
}

func TestNewExtractorSupports7zArchives(t *testing.T) {
	extractor := NewExtractor("tool.7z", "tool", NewBinaryChooser("tool"))
	if _, ok := extractor.(*ArchiveExtractor); !ok {
		t.Fatalf("expected 7z extractor to use archive extractor, got %T", extractor)
	}
}

func TestExtractFileRequestsMultipleMatches(t *testing.T) {
	if extractAllFromFileSpec("README") {
		t.Fatal("expected single file spec to keep single extraction mode")
	}
	if !extractAllFromFileSpec("README,LICENSE") {
		t.Fatal("expected comma-separated file spec to enable multi extraction mode")
	}
	if !extractAllFromFileSpec("*.exe") {
		t.Fatal("expected glob file spec to enable multi extraction mode")
	}
}

func TestArchiveDirectoryExtractRejectsPathTraversal(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if _, err := zw.Create("safe/"); err != nil {
		t.Fatalf("create zip dir: %v", err)
	}
	w, err := zw.Create("safe/../../evil.txt")
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	if _, err := w.Write([]byte("evil")); err != nil {
		t.Fatalf("write zip file: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	chooser, err := NewFileChooser("*")
	if err != nil {
		t.Fatalf("NewFileChooser: %v", err)
	}
	extractor := NewArchiveExtractor(chooser, NewZipArchive, nil)
	file, _, err := extractor.Extract(buf.Bytes(), true)
	if err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("expected unsafe archive path error, got %v", err)
	}

	root := t.TempDir()
	if file.Extract != nil {
		t.Fatal("expected no extract function for unsafe archive")
	}
	if _, statErr := os.Stat(filepath.Join(root, "evil.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("expected traversal output to be absent, stat error: %v", statErr)
	}
}

func TestTarArchiveNextRejectsPathTraversal(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: "../evil.txt", Mode: 0o644, Size: int64(len("evil"))}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write([]byte("evil")); err != nil {
		t.Fatalf("write tar file: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	ar, err := NewTarArchive(buf.Bytes(), func(r io.Reader) (io.Reader, error) { return r, nil })
	if err != nil {
		t.Fatalf("NewTarArchive: %v", err)
	}
	_, err = ar.Next()
	if err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("expected unsafe archive path error, got %v", err)
	}
}

func TestZipArchiveNextRejectsPathTraversal(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("../evil.txt")
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	if _, err := w.Write([]byte("evil")); err != nil {
		t.Fatalf("write zip file: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	ar, err := NewZipArchive(buf.Bytes(), nil)
	if err != nil {
		t.Fatalf("NewZipArchive: %v", err)
	}
	_, err = ar.Next()
	if err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("expected unsafe archive path error, got %v", err)
	}
}

func TestAssetDetectorSupportsRegexInclude(t *testing.T) {
	re, err := compileAssetRegex(`\.deb$`)
	if err != nil {
		t.Fatalf("compileAssetRegex: %v", err)
	}
	d := &assetDetector{Asset: `\.deb$`, Regex: re}

	got, candidates, err := d.Detect([]string{
		"https://example.com/pkg_1.0.0_amd64.deb",
		"https://example.com/pkg_1.0.0_amd64.rpm",
	})
	if err != nil {
		t.Fatalf("Detect(): %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected no candidates, got %#v", candidates)
	}
	if got != "https://example.com/pkg_1.0.0_amd64.deb" {
		t.Fatalf("expected deb asset to match, got %q", got)
	}
}

func TestAssetDetectorSupportsRegexExclude(t *testing.T) {
	re, err := compileAssetRegex(`\.deb$`)
	if err != nil {
		t.Fatalf("compileAssetRegex: %v", err)
	}
	d := &assetDetector{Asset: `\.deb$`, Anti: true, Regex: re}

	got, candidates, err := d.Detect([]string{
		"https://example.com/pkg_1.0.0_amd64.deb",
		"https://example.com/pkg_1.0.0_amd64.rpm",
	})
	if err != nil {
		t.Fatalf("Detect(): %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected no candidates, got %#v", candidates)
	}
	if got != "https://example.com/pkg_1.0.0_amd64.rpm" {
		t.Fatalf("expected rpm asset to remain after exclude, got %q", got)
	}
}

func TestAssetDetectorMatchesPlainFilterCaseInsensitive(t *testing.T) {
	d := &assetDetector{Asset: "setup"}

	got, candidates, err := d.Detect([]string{
		"https://example.com/WinMerge-2.16.56-x64-Setup.exe",
		"https://example.com/WinMerge-2.16.56-x64.zip",
	})
	if err != nil {
		t.Fatalf("Detect(): %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected no candidates, got %#v", candidates)
	}
	if got != "https://example.com/WinMerge-2.16.56-x64-Setup.exe" {
		t.Fatalf("expected setup asset to match, got %q", got)
	}
}
