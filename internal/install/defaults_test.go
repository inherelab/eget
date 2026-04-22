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
}
