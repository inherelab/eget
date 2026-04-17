package install

import (
	"errors"
	"testing"
)

type fakeDetector struct {
	name string
}

func (f *fakeDetector) Detect(assets []string) (string, []string, error) {
	return f.name, nil, nil
}

type fakeVerifier struct {
	name string
}

func (f *fakeVerifier) Verify(b []byte) error {
	return nil
}

type fakeChooser struct {
	name string
}

type fakeExtractor struct {
	name string
}

func TestSelectDetector(t *testing.T) {
	svc := NewService()
	svc.AllDetectorFactory = func() Detector {
		return &fakeDetector{name: "all"}
	}
	svc.SystemDetectorFactory = func(goos, goarch string) (Detector, error) {
		return &fakeDetector{name: goos + "/" + goarch}, nil
	}
	svc.AssetDetectorFactory = func(asset string, anti bool) Detector {
		name := asset
		if anti {
			name = "^" + name
		}
		return &fakeDetector{name: name}
	}
	svc.DetectorChainFactory = func(detectors []Detector, system Detector) Detector {
		return &fakeDetector{name: "chain"}
	}

	detector, err := svc.SelectDetector(&Options{System: "all"})
	if err != nil {
		t.Fatalf("SelectDetector(all): %v", err)
	}
	if got := detector.(*fakeDetector).name; got != "all" {
		t.Fatalf("SelectDetector(all) = %q, want %q", got, "all")
	}

	detector, err = svc.SelectDetector(&Options{System: "linux/amd64", Asset: []string{"cli", "^arm"}})
	if err != nil {
		t.Fatalf("SelectDetector(custom): %v", err)
	}
	if got := detector.(*fakeDetector).name; got != "chain" {
		t.Fatalf("SelectDetector(custom) = %q, want %q", got, "chain")
	}
}

func TestSelectVerifier(t *testing.T) {
	svc := NewService()
	svc.Sha256VerifierFactory = func(expected string) (Verifier, error) {
		if expected == "bad" {
			return nil, errors.New("bad verifier")
		}
		return &fakeVerifier{name: "verify:" + expected}, nil
	}
	svc.Sha256AssetVerifierFactory = func(assetURL string) Verifier {
		return &fakeVerifier{name: "asset:" + assetURL}
	}
	svc.Sha256PrinterFactory = func() Verifier {
		return &fakeVerifier{name: "printer"}
	}
	svc.NoVerifierFactory = func() Verifier {
		return &fakeVerifier{name: "noop"}
	}

	verifier, err := svc.SelectVerifier("sum.txt", &Options{Verify: "abc"})
	if err != nil {
		t.Fatalf("SelectVerifier(verify): %v", err)
	}
	if got := verifier.(*fakeVerifier).name; got != "verify:abc" {
		t.Fatalf("SelectVerifier(verify) = %q", got)
	}

	verifier, err = svc.SelectVerifier("sum.txt", &Options{})
	if err != nil {
		t.Fatalf("SelectVerifier(asset): %v", err)
	}
	if got := verifier.(*fakeVerifier).name; got != "asset:sum.txt" {
		t.Fatalf("SelectVerifier(asset) = %q", got)
	}

	verifier, err = svc.SelectVerifier("", &Options{Hash: true})
	if err != nil {
		t.Fatalf("SelectVerifier(hash): %v", err)
	}
	if got := verifier.(*fakeVerifier).name; got != "printer" {
		t.Fatalf("SelectVerifier(hash) = %q", got)
	}

	verifier, err = svc.SelectVerifier("", &Options{})
	if err != nil {
		t.Fatalf("SelectVerifier(noop): %v", err)
	}
	if got := verifier.(*fakeVerifier).name; got != "noop" {
		t.Fatalf("SelectVerifier(noop) = %q", got)
	}
}

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
