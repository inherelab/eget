package install

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sourcegithub "github.com/inherelab/eget/internal/source/github"
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

type fakeHTTPGetterFunc func(url string) (*http.Response, error)

func (f fakeHTTPGetterFunc) Get(url string) (*http.Response, error) {
	return f(url)
}

func TestNewDefaultServiceWiring(t *testing.T) {
	svc := NewDefaultService(
		fakeHTTPGetterFunc(func(url string) (*http.Response, error) {
			if url == "https://example.com/tool.sha256" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08")),
				}, nil
			}
			return nil, errors.New("unexpected url")
		}),
		func(tool, output string) time.Time { return time.Unix(123, 0) },
	)

	finder, tool, err := svc.SelectFinder("inhere/markview", &Options{UpgradeOnly: true})
	if err != nil {
		t.Fatalf("SelectFinder(default): %v", err)
	}
	if tool != "markview" {
		t.Fatalf("tool = %q, want %q", tool, "markview")
	}
	if _, ok := finder.(*sourcegithub.AssetFinder); !ok {
		t.Fatalf("finder type = %T, want *github.AssetFinder", finder)
	}

	detector, err := svc.SelectDetector(&Options{System: "linux/amd64", Asset: []string{"cli"}})
	if err != nil {
		t.Fatalf("SelectDetector(default): %v", err)
	}
	if detector == nil {
		t.Fatal("expected detector")
	}

	verifier, err := svc.SelectVerifier("https://example.com/tool.sha256", &Options{})
	if err != nil {
		t.Fatalf("SelectVerifier(default): %v", err)
	}
	if err := verifier.Verify([]byte("test")); err != nil {
		t.Fatalf("Verify(default): %v", err)
	}

	extractor, err := SelectExtractorAs[Extractor](svc, "https://example.com/tool.tar.gz", "tool", &Options{})
	if err != nil {
		t.Fatalf("SelectExtractor(default): %v", err)
	}
	if extractor == nil {
		t.Fatal("expected extractor")
	}
}

func TestSelectFinder(t *testing.T) {
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "tool.tar.gz")
	if err := os.WriteFile(localFile, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	svc := NewService()
	svc.GitHubGetter = fakeHTTPGetterFunc(func(url string) (*http.Response, error) {
		return nil, nil
	})
	svc.GitHubGetterFactory = func(opts Options) sourcegithub.HTTPGetter {
		return fakeHTTPGetterFunc(func(url string) (*http.Response, error) {
			if opts.ProxyURL != "http://127.0.0.1:7890" {
				t.Fatalf("expected proxy url to propagate to finder getter, got %q", opts.ProxyURL)
			}
			return nil, nil
		})
	}
	svc.BinaryModTime = func(tool, output string) time.Time {
		return time.Unix(123, 0)
	}

	t.Run("repo target", func(t *testing.T) {
		opts := &Options{Tag: "v1.2.3", ProxyURL: "http://127.0.0.1:7890"}
		finder, tool, err := svc.SelectFinder("inhere/markview", opts)
		if err != nil {
			t.Fatalf("SelectFinder(repo): %v", err)
		}
		if tool != "markview" {
			t.Fatalf("tool = %q, want %q", tool, "markview")
		}
		got, ok := finder.(*sourcegithub.AssetFinder)
		if !ok {
			t.Fatalf("finder type = %T, want *github.AssetFinder", finder)
		}
		if got.Repo != "inhere/markview" || got.Tag != "tags/v1.2.3" {
			t.Fatalf("finder = %+v", got)
		}
	})

	t.Run("github url", func(t *testing.T) {
		opts := &Options{Source: true, Tag: "main"}
		finder, tool, err := svc.SelectFinder("https://github.com/inhere/markview", opts)
		if err != nil {
			t.Fatalf("SelectFinder(github url): %v", err)
		}
		if tool != "markview" {
			t.Fatalf("tool = %q, want %q", tool, "markview")
		}
		got, ok := finder.(*sourcegithub.SourceFinder)
		if !ok {
			t.Fatalf("finder type = %T, want *github.SourceFinder", finder)
		}
		if got.Repo != "inhere/markview" || got.Tag != "main" || got.Tool != "markview" {
			t.Fatalf("finder = %+v", got)
		}
	})

	t.Run("direct url", func(t *testing.T) {
		opts := &Options{}
		finder, tool, err := svc.SelectFinder("https://example.com/tool.tar.gz", opts)
		if err != nil {
			t.Fatalf("SelectFinder(direct): %v", err)
		}
		if tool != "" {
			t.Fatalf("tool = %q, want empty", tool)
		}
		got, ok := finder.(*DirectAssetFinder)
		if !ok {
			t.Fatalf("finder type = %T, want *DirectAssetFinder", finder)
		}
		if got.URL != "https://example.com/tool.tar.gz" {
			t.Fatalf("URL = %q", got.URL)
		}
		if opts.System != "all" {
			t.Fatalf("opts.System = %q, want %q", opts.System, "all")
		}
	})

	t.Run("local file", func(t *testing.T) {
		opts := &Options{}
		finder, tool, err := svc.SelectFinder(localFile, opts)
		if err != nil {
			t.Fatalf("SelectFinder(local): %v", err)
		}
		if tool != "" {
			t.Fatalf("tool = %q, want empty", tool)
		}
		got, ok := finder.(*DirectAssetFinder)
		if !ok {
			t.Fatalf("finder type = %T, want *DirectAssetFinder", finder)
		}
		if got.URL != localFile {
			t.Fatalf("URL = %q, want %q", got.URL, localFile)
		}
		if opts.System != "all" {
			t.Fatalf("opts.System = %q, want %q", opts.System, "all")
		}
	})

	t.Run("invalid target", func(t *testing.T) {
		if _, _, err := svc.SelectFinder("invalid-target", &Options{}); err == nil {
			t.Fatal("expected invalid target error")
		}
	})
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
	svc.Sha256AssetVerifierFactory = func(assetURL string, opts Options) Verifier {
		_ = opts
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
