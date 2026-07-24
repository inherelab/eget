package install

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	"github.com/inherelab/eget/internal/cachemirror"
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

type chooserRecorder struct {
	value any
}

type fakeExtractor struct {
	name string
}

func (f *fakeExtractor) Extract([]byte, bool) (ExtractedFile, []ExtractedFile, error) {
	return ExtractedFile{}, nil, nil
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

func TestDefaultChecksumVerifierUsesCacheMirror(t *testing.T) {
	cacheDir := t.TempDir()
	originHit := false
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		originHit = true
		_, _ = w.Write([]byte("9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"))
	}))
	defer origin.Close()

	checksumURL := origin.URL + "/tool.zip.sha256"
	cachePath := CacheFilePath(cacheDir, checksumURL)
	rel, err := cachemirror.RelPath(cacheDir, cachePath)
	assert.NoErr(t, err)
	wantPath := "/download/" + cachemirror.KeyForRelPath(rel)
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Eq(t, wantPath, r.URL.Path)
		_, _ = w.Write([]byte("9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"))
	}))
	defer mirror.Close()

	verifier := NewDefaultService(nil, nil).Sha256AssetVerifierFactory(checksumURL, Options{
		CacheDir: cacheDir,
		CacheMirror: cachemirror.Options{
			Enable: true, URL: mirror.URL, Fallback: false,
		},
	})
	assert.NoErr(t, verifier.Verify([]byte("test")))
	assert.False(t, originHit)
}
