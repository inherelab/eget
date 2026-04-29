package install

import "testing"

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
