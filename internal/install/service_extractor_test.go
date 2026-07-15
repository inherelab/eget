package install

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestSelectExtractor(t *testing.T) {
	svc := NewService()
	svc.DownloadOnlyExtractorFactory = func(name string) any {
		return &fakeExtractor{name: "download:" + name}
	}
	svc.GlobChooserFactory = func(pattern string) (any, error) {
		if pattern == "bad[" {
			return nil, errors.New("bad glob")
		}
		return &fakeChooser{name: "glob:" + pattern}, nil
	}
	svc.BinaryChooserFactory = func(tool string) any {
		return &fakeChooser{name: "binary:" + tool}
	}
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		ch := chooser.(*fakeChooser)
		return &fakeExtractor{name: filename + "|" + tool + "|" + ch.name}
	}

	extractor, err := svc.SelectExtractor("https://example.com/tool.tar.gz", "tool", &Options{DownloadOnly: true})
	if err != nil {
		t.Fatalf("SelectExtractor(download): %v", err)
	}
	if got := extractor.(*fakeExtractor).name; got != "download:tool.tar.gz" {
		t.Fatalf("SelectExtractor(download) = %q", got)
	}

	extractor, err = svc.SelectExtractor("https://example.com/tool.tar.gz", "tool", &Options{ExtractFile: "LICENSE"})
	if err != nil {
		t.Fatalf("SelectExtractor(glob): %v", err)
	}
	if got := extractor.(*fakeExtractor).name; got != "tool.tar.gz|tool|glob:LICENSE" {
		t.Fatalf("SelectExtractor(glob) = %q", got)
	}

	extractor, err = svc.SelectExtractor("https://example.com/tool.tar.gz", "tool", &Options{})
	if err != nil {
		t.Fatalf("SelectExtractor(binary): %v", err)
	}
	if got := extractor.(*fakeExtractor).name; got != "tool.tar.gz|tool|binary:tool" {
		t.Fatalf("SelectExtractor(binary) = %q", got)
	}
}

func TestSelectExtractorUsesRawFileForInstallerMode(t *testing.T) {
	svc := NewService()
	svc.DownloadOnlyExtractorFactory = func(name string) any {
		return &fakeExtractor{name: "raw:" + name}
	}
	svc.BinaryChooserFactory = func(tool string) any {
		return &fakeChooser{name: tool}
	}
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		return &fakeExtractor{name: "archive:" + filename}
	}

	extractor, err := svc.SelectExtractor("https://example.com/PowerShell-7.6.3-win-x64.msi", "PowerShell", &Options{InstallMode: InstallModeInstaller})

	assert.NoErr(t, err)
	assert.Eq(t, "raw:PowerShell-7.6.3-win-x64.msi", extractor.(*fakeExtractor).name)
}

func TestSelectExtractorUsesSystem7zForSevenZipWhenAvailable(t *testing.T) {
	svc := NewService()
	svc.BinaryChooserFactory = func(tool string) any {
		return NewBinaryChooser(tool)
	}
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		return &fakeExtractor{name: "go:" + filename}
	}
	svc.System7zPathResolver = func(configured string) string {
		if configured != "C:/Tools/7z.exe" {
			t.Fatalf("expected configured 7z path to propagate, got %q", configured)
		}
		return "C:/Tools/7z.exe"
	}
	svc.System7zExtractorFactory = func(filename, tool string, chooser Chooser, exe string) Extractor {
		return &fakeExtractor{name: "system7z:" + filepath.Base(filename)}
	}

	extractor, err := svc.SelectExtractor("https://example.com/tool.7z", "tool", &Options{Sys7zPath: "C:/Tools/7z.exe"})
	if err != nil {
		t.Fatalf("SelectExtractor(system 7z): %v", err)
	}
	if got := extractor.(*fakeExtractor).name; got != "system7z:tool.7z" {
		t.Fatalf("expected system 7z extractor, got %q", got)
	}
}

func TestSelectExtractorFallsBackToGoExtractorWithoutSystem7z(t *testing.T) {
	svc := NewService()
	svc.BinaryChooserFactory = func(tool string) any {
		return NewBinaryChooser(tool)
	}
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		return &fakeExtractor{name: "go:" + filename}
	}
	svc.System7zPathResolver = func(configured string) string {
		return ""
	}

	extractor, err := svc.SelectExtractor("https://example.com/tool.7z", "tool", &Options{})
	if err != nil {
		t.Fatalf("SelectExtractor(go fallback): %v", err)
	}
	if got := extractor.(*fakeExtractor).name; got != "go:tool.7z" {
		t.Fatalf("expected Go extractor fallback, got %q", got)
	}
}

func TestSelectExtractorKeepsTarGzOnGoExtractorEvenWithSystem7z(t *testing.T) {
	svc := NewService()
	svc.BinaryChooserFactory = func(tool string) any {
		return NewBinaryChooser(tool)
	}
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		return &fakeExtractor{name: "go:" + filename}
	}
	svc.System7zPathResolver = func(configured string) string {
		return "C:/Tools/7z.exe"
	}
	svc.System7zExtractorFactory = func(filename, tool string, chooser Chooser, exe string) Extractor {
		return &fakeExtractor{name: "system7z:" + filepath.Base(filename)}
	}

	extractor, err := svc.SelectExtractor("https://example.com/tool.tar.gz", "tool", &Options{})
	if err != nil {
		t.Fatalf("SelectExtractor(tar.gz): %v", err)
	}
	if got := extractor.(*fakeExtractor).name; got != "go:tool.tar.gz" {
		t.Fatalf("expected tar.gz to stay on Go extractor, got %q", got)
	}
}

func TestSelectExtractorDoesNotUseSystem7zForPureDownloadOnly(t *testing.T) {
	svc := NewService()
	svc.DownloadOnlyExtractorFactory = func(name string) any {
		return &fakeExtractor{name: "download:" + name}
	}
	svc.System7zPathResolver = func(configured string) string {
		return "C:/Tools/7z.exe"
	}

	extractor, err := svc.SelectExtractor("https://example.com/tool.7z", "tool", &Options{DownloadOnly: true})
	if err != nil {
		t.Fatalf("SelectExtractor(download-only): %v", err)
	}
	if got := extractor.(*fakeExtractor).name; got != "download:tool.7z" {
		t.Fatalf("expected pure download-only extractor, got %q", got)
	}
}

func TestSelectExtractorTreatsDownloadWithExtractFileAsArchiveExtraction(t *testing.T) {
	svc := NewService()
	rec := &chooserRecorder{}
	svc.GlobChooserFactory = func(pattern string) (any, error) {
		return &fakeChooser{name: "glob:" + pattern}, nil
	}
	svc.BinaryChooserFactory = func(tool string) any {
		return &fakeChooser{name: "binary:" + tool}
	}
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		rec.value = chooser
		ch := chooser.(*fakeChooser)
		return &fakeExtractor{name: filename + "|" + tool + "|" + ch.name}
	}

	extractor, err := svc.SelectExtractor("https://example.com/tool.tar.gz", "tool", &Options{
		DownloadOnly: true,
		ExtractFile:  "LICENSE",
	})
	if err != nil {
		t.Fatalf("SelectExtractor(download with file): %v", err)
	}
	if got := extractor.(*fakeExtractor).name; got != "tool.tar.gz|tool|glob:LICENSE" {
		t.Fatalf("SelectExtractor(download with file) = %q", got)
	}
}

func TestSelectExtractorUsesURLPathFilenameWithoutQuery(t *testing.T) {
	svc := NewService()
	svc.GlobChooserFactory = func(pattern string) (any, error) {
		return &fakeChooser{name: "glob:" + pattern}, nil
	}
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		ch := chooser.(*fakeChooser)
		return &fakeExtractor{name: filename + "|" + tool + "|" + ch.name}
	}

	extractor, err := svc.SelectExtractor("https://example.com/artifacts/tool.zip?job=build_linux", "tool", &Options{ExtractFile: "*.zip"})
	if err != nil {
		t.Fatalf("SelectExtractor(query filename): %v", err)
	}
	if got := extractor.(*fakeExtractor).name; got != "tool.zip|tool|glob:*.zip" {
		t.Fatalf("SelectExtractor(query filename) = %q", got)
	}
}

func TestSelectExtractorTreatsDownloadWithAllAsArchiveExtraction(t *testing.T) {
	svc := NewService()
	svc.GlobChooserFactory = func(pattern string) (any, error) {
		return &fakeChooser{name: "glob:" + pattern}, nil
	}
	svc.BinaryChooserFactory = func(tool string) any {
		return &fakeChooser{name: "binary:" + tool}
	}
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		ch := chooser.(*fakeChooser)
		return &fakeExtractor{name: filename + "|" + tool + "|" + ch.name}
	}

	extractor, err := svc.SelectExtractor("https://example.com/tool.tar.gz", "tool", &Options{
		DownloadOnly: true,
		All:          true,
	})
	if err != nil {
		t.Fatalf("SelectExtractor(download with all): %v", err)
	}
	if got := extractor.(*fakeExtractor).name; got != "tool.tar.gz|tool|glob:*" {
		t.Fatalf("SelectExtractor(download with all) = %q", got)
	}
}

func TestSelectExtractorRequiresSystem7zForExeExtractAll(t *testing.T) {
	svc := NewDefaultService(nil, nil)
	svc.System7zPathResolver = func(configured string) string {
		return ""
	}

	_, err := SelectExtractorAs[Extractor](svc, "https://example.com/qbittorrent_5.2.0_x64_setup.exe", "qbittorrent", &Options{
		All: true,
	})
	if err == nil {
		t.Fatal("expected exe extract-all without system 7z to fail")
	}
	if !strings.Contains(err.Error(), "system 7z is required") {
		t.Fatalf("expected missing system 7z error, got %v", err)
	}
}

func TestSelectExtractorRequiresSystem7zForExeFilePattern(t *testing.T) {
	svc := NewDefaultService(nil, nil)
	svc.System7zPathResolver = func(configured string) string {
		return ""
	}

	_, err := SelectExtractorAs[Extractor](svc, "https://example.com/qbittorrent_5.2.0_x64_setup.exe", "qbittorrent", &Options{
		ExtractFile: "qbittorrent.exe,*.dll",
	})
	if err == nil {
		t.Fatal("expected exe file pattern without system 7z to fail")
	}
	if !strings.Contains(err.Error(), "system 7z is required") {
		t.Fatalf("expected missing system 7z error, got %v", err)
	}
}

func TestSelectExtractorUsesSystem7zForExeExtractAll(t *testing.T) {
	svc := NewService()
	svc.GlobChooserFactory = func(pattern string) (any, error) {
		return NewFileChooser(pattern)
	}
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		return &fakeExtractor{name: "go:" + filename}
	}
	svc.System7zPathResolver = func(configured string) string {
		return "C:/Tools/7z.exe"
	}
	svc.System7zExtractorFactory = func(filename, tool string, chooser Chooser, exe string) Extractor {
		return &fakeExtractor{name: "system7z:" + filepath.Base(filename)}
	}

	extractor, err := svc.SelectExtractor("https://example.com/setup.exe", "setup", &Options{All: true})
	if err != nil {
		t.Fatalf("SelectExtractor(exe extract-all): %v", err)
	}
	if got := extractor.(*fakeExtractor).name; got != "system7z:setup.exe" {
		t.Fatalf("expected system 7z extractor, got %q", got)
	}
}

func TestSelectExtractorUsesSystem7zForExeFilePattern(t *testing.T) {
	svc := NewService()
	svc.GlobChooserFactory = func(pattern string) (any, error) {
		return NewFileChooser(pattern)
	}
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		return &fakeExtractor{name: "go:" + filename}
	}
	svc.System7zPathResolver = func(configured string) string {
		return "C:/Tools/7z.exe"
	}
	svc.System7zExtractorFactory = func(filename, tool string, chooser Chooser, exe string) Extractor {
		return &fakeExtractor{name: "system7z:" + filepath.Base(filename)}
	}

	extractor, err := svc.SelectExtractor("https://example.com/setup.exe", "setup", &Options{ExtractFile: "bin/*.exe"})
	if err != nil {
		t.Fatalf("SelectExtractor(exe file pattern): %v", err)
	}
	if got := extractor.(*fakeExtractor).name; got != "system7z:setup.exe" {
		t.Fatalf("expected system 7z extractor, got %q", got)
	}
}

func TestSelectExtractorPrefersExplicitFilePatternsOverAll(t *testing.T) {
	svc := NewService()
	svc.GlobChooserFactory = func(pattern string) (any, error) {
		return &fakeChooser{name: "glob:" + pattern}, nil
	}
	svc.BinaryChooserFactory = func(tool string) any {
		return &fakeChooser{name: "binary:" + tool}
	}
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		ch := chooser.(*fakeChooser)
		return &fakeExtractor{name: filename + "|" + tool + "|" + ch.name}
	}

	extractor, err := svc.SelectExtractor("https://example.com/tool.tar.gz", "tool", &Options{
		DownloadOnly: true,
		ExtractFile:  "README,LICENSE",
		All:          true,
	})
	if err != nil {
		t.Fatalf("SelectExtractor(download with explicit file patterns): %v", err)
	}
	if got := extractor.(*fakeExtractor).name; got != "tool.tar.gz|tool|glob:README,LICENSE" {
		t.Fatalf("SelectExtractor(download with explicit file patterns) = %q", got)
	}
}
