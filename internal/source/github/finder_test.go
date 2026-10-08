package github

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fakeGetter struct {
	responses map[string]*http.Response
	errs      map[string]error
}

func (f *fakeGetter) Get(url string) (*http.Response, error) {
	if err := f.errs[url]; err != nil {
		return nil, err
	}
	if resp, ok := f.responses[url]; ok {
		return resp, nil
	}
	return nil, fmt.Errorf("unexpected url: %s", url)
}

func jsonResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestAssetFinderFind(t *testing.T) {
	getter := &fakeGetter{
		responses: map[string]*http.Response{
			"https://api.github.com/repos/inhere/markview/releases/latest": jsonResponse(http.StatusOK, `{"tag_name":"v1.2.3","assets":[{"id":100,"size":12,"updated_at":"2026-04-22T10:00:00Z","digest":"sha256:abc","browser_download_url":"https://example.com/tool.tar.gz"}],"created_at":"2026-04-18T00:00:00Z"}`),
		},
	}
	finder := NewAssetFinder("inhere/markview", "latest", false, time.Time{})
	finder.Getter = getter

	assets, err := finder.Find()
	if err != nil {
		t.Fatalf("Find(): %v", err)
	}
	if len(assets) != 1 || assets[0] != "https://example.com/tool.tar.gz" {
		t.Fatalf("assets = %#v", assets)
	}
	if finder.ReleaseVersion() != "v1.2.3" {
		t.Fatalf("ReleaseVersion() = %q", finder.ReleaseVersion())
	}
	id, size, updatedAt, digest, ok := finder.AssetMetadata("https://example.com/tool.tar.gz")
	if !ok {
		t.Fatal("expected selected asset metadata")
	}
	if id != 100 || size != 12 || digest != "sha256:abc" {
		t.Fatalf("unexpected asset metadata id=%d size=%d digest=%q", id, size, digest)
	}
	if !updatedAt.Equal(time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected asset updated_at: %s", updatedAt)
	}
}

func TestAssetFinderFindMatchFallback(t *testing.T) {
	getter := &fakeGetter{
		responses: map[string]*http.Response{
			"https://api.github.com/repos/inhere/markview/releases/tags/v1.2.3": jsonResponse(http.StatusNotFound, `{"message":"not found"}`),
			"https://api.github.com/repos/inhere/markview/releases?page=1":      jsonResponse(http.StatusOK, `[{"tag_name":"v1.2.3-rc1","created_at":"2026-04-18T00:00:00Z","assets":[{"browser_download_url":"https://example.com/match.tar.gz"}]}]`),
		},
	}
	finder := NewAssetFinder("inhere/markview", "tags/v1.2.3", false, time.Time{})
	finder.Getter = getter

	assets, err := finder.Find()
	if err != nil {
		t.Fatalf("Find() fallback: %v", err)
	}
	if len(assets) != 1 || assets[0] != "https://example.com/match.tar.gz" {
		t.Fatalf("assets = %#v", assets)
	}
}

func TestAssetFinderFindPrereleaseLatest(t *testing.T) {
	getter := &fakeGetter{
		responses: map[string]*http.Response{
			"https://api.github.com/repos/inhere/markview/releases":                 jsonResponse(http.StatusOK, `[{"tag_name":"v2.0.0-rc1"}]`),
			"https://api.github.com/repos/inhere/markview/releases/tags/v2.0.0-rc1": jsonResponse(http.StatusOK, `{"assets":[{"browser_download_url":"https://example.com/rc1.tar.gz"}],"created_at":"2026-04-18T00:00:00Z"}`),
		},
	}
	finder := NewAssetFinder("inhere/markview", "latest", true, time.Time{})
	finder.Getter = getter

	assets, err := finder.Find()
	if err != nil {
		t.Fatalf("Find() prerelease latest: %v", err)
	}
	if finder.Tag != "tags/v2.0.0-rc1" {
		t.Fatalf("Tag = %q", finder.Tag)
	}
	if len(assets) != 1 || assets[0] != "https://example.com/rc1.tar.gz" {
		t.Fatalf("assets = %#v", assets)
	}
}

func TestAssetFinderErrNoUpgrade(t *testing.T) {
	getter := &fakeGetter{
		responses: map[string]*http.Response{
			"https://api.github.com/repos/inhere/markview/releases/latest": jsonResponse(http.StatusOK, `{"assets":[{"browser_download_url":"https://example.com/tool.tar.gz"}],"created_at":"2026-04-17T00:00:00Z"}`),
		},
	}
	finder := NewAssetFinder("inhere/markview", "latest", false, time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC))
	finder.Getter = getter

	_, err := finder.Find()
	if err != ErrNoUpgrade {
		t.Fatalf("Find() err = %v, want ErrNoUpgrade", err)
	}
}

func TestAssetFinderGetterRequired(t *testing.T) {
	finder := NewAssetFinder("inhere/markview", "latest", false, time.Time{})
	if _, err := finder.Find(); err == nil {
		t.Fatal("expected getter required error")
	}
	if _, err := finder.FindMatch(); err == nil {
		t.Fatal("expected getter required error for FindMatch")
	}
}

func TestAssetFinderVerboseLogsResponseSummary(t *testing.T) {
	var verbose bytes.Buffer
	origVerboseEnabled := verboseEnabled
	origVerboseWriter := verboseWriter
	defer func() {
		verboseEnabled = origVerboseEnabled
		verboseWriter = origVerboseWriter
	}()
	SetVerbose(true, &verbose)

	getter := &fakeGetter{
		responses: map[string]*http.Response{
			"https://api.github.com/repos/inhere/markview/releases/latest": jsonResponse(http.StatusOK, `{"assets":[{"browser_download_url":"https://example.com/tool.tar.gz"}],"created_at":"2026-04-18T00:00:00Z"}`),
		},
	}
	finder := NewAssetFinder("inhere/markview", "latest", false, time.Time{})
	finder.Getter = getter

	_, err := finder.Find()
	if err != nil {
		t.Fatalf("Find(): %v", err)
	}

	got := verbose.String()
	if !strings.Contains(got, "[verbose] github finder request: https://api.github.com/repos/inhere/markview/releases/latest") {
		t.Fatalf("expected verbose finder request log, got %q", got)
	}
	if !strings.Contains(got, "[verbose] github finder assets: 1") {
		t.Fatalf("expected verbose finder asset count, got %q", got)
	}
}

func TestAssetFinderFindMatchAnchorsPlainTag(t *testing.T) {
	// The older behaviour used strings.Contains, so the first release (which
	// merely embeds the filter text) would win. An anchored prefix must skip it.
	getter := &fakeGetter{
		responses: map[string]*http.Response{
			"https://api.github.com/repos/trycua/cua/releases/tags/cua-driver-rs-v": jsonResponse(http.StatusNotFound, `{"message":"not found"}`),
			"https://api.github.com/repos/trycua/cua/releases?page=1": jsonResponse(http.StatusOK, `[
				{"tag_name":"x-cua-driver-rs-v9.9.9","created_at":"2026-10-06T00:00:00Z","assets":[{"browser_download_url":"https://example.com/wrong.tar.gz"}]},
				{"tag_name":"cua-driver-rs-v0.34.0","created_at":"2026-10-05T00:00:00Z","assets":[{"browser_download_url":"https://example.com/right.tar.gz"}]}
			]`),
		},
	}
	finder := NewAssetFinder("trycua/cua", "tags/cua-driver-rs-v", false, time.Time{})
	finder.Getter = getter

	assets, err := finder.Find()
	if err != nil {
		t.Fatalf("Find() anchored match: %v", err)
	}
	if len(assets) != 1 || assets[0] != "https://example.com/right.tar.gz" {
		t.Fatalf("assets = %#v", assets)
	}
	if finder.ReleaseVersion() != "cua-driver-rs-v0.34.0" {
		t.Fatalf("ReleaseVersion() = %q", finder.ReleaseVersion())
	}
}

func TestAssetFinderFindMatchPatternSkipsExactLookup(t *testing.T) {
	// No releases/tags/<pattern> response is registered: an exact-tag lookup for
	// a pattern would hit fakeGetter's "unexpected url" error, so this also
	// asserts the pattern fast-path.
	getter := &fakeGetter{
		responses: map[string]*http.Response{
			"https://api.github.com/repos/trycua/cua/releases?page=1": jsonResponse(http.StatusOK, `[
				{"tag_name":"nightly-lume-v0.6.2-nightly.20261008","created_at":"2026-10-08T04:59:00Z","assets":[{"browser_download_url":"https://example.com/lume.tar.gz"}]},
				{"tag_name":"cua-driver-rs-v0.34.0","created_at":"2026-10-05T22:30:00Z","assets":[{"browser_download_url":"https://example.com/driver.zip"}]}
			]`),
		},
	}
	finder := NewAssetFinder("trycua/cua", "tags/PRE:cua-driver-rs-v", false, time.Time{})
	finder.Getter = getter

	assets, err := finder.Find()
	if err != nil {
		t.Fatalf("Find() pattern: %v", err)
	}
	if finder.ReleaseVersion() != "cua-driver-rs-v0.34.0" {
		t.Fatalf("ReleaseVersion() = %q", finder.ReleaseVersion())
	}
	if len(assets) != 1 || assets[0] != "https://example.com/driver.zip" {
		t.Fatalf("assets = %#v", assets)
	}
}

func TestAssetFinderFindMatchPrereleaseFallback(t *testing.T) {
	// A product whose every release is marked prerelease must still resolve
	// without -p (monorepos do this to protect the repository-wide Latest).
	getter := &fakeGetter{
		responses: map[string]*http.Response{
			"https://api.github.com/repos/trycua/cua/releases?page=1": jsonResponse(http.StatusOK, `[{"tag_name":"cua-driver-rs-v0.34.0","prerelease":true,"created_at":"2026-10-05T22:30:00Z","assets":[{"browser_download_url":"https://example.com/driver.zip"}]}]`),
		},
	}
	finder := NewAssetFinder("trycua/cua", "tags/PRE:cua-driver-rs-v", false, time.Time{})
	finder.Getter = getter

	assets, err := finder.Find()
	if err != nil {
		t.Fatalf("Find() prerelease fallback: %v", err)
	}
	if len(assets) != 1 || assets[0] != "https://example.com/driver.zip" {
		t.Fatalf("assets = %#v", assets)
	}
}

func TestAssetFinderFindMatchPrefersStableOverPrerelease(t *testing.T) {
	releases := `[
		{"tag_name":"app-v2.0.0-rc1","prerelease":true,"created_at":"2026-10-06T00:00:00Z","assets":[{"browser_download_url":"https://example.com/rc.tar.gz"}]},
		{"tag_name":"app-v1.9.0","prerelease":false,"created_at":"2026-10-01T00:00:00Z","assets":[{"browser_download_url":"https://example.com/stable.tar.gz"}]}
	]`
	// Each Find() needs its own response: a response body can only be read once.
	newGetter := func() *fakeGetter {
		return &fakeGetter{
			responses: map[string]*http.Response{
				"https://api.github.com/repos/acme/app/releases?page=1": jsonResponse(http.StatusOK, releases),
			},
		}
	}

	finder := NewAssetFinder("acme/app", "tags/PRE:app-v", false, time.Time{})
	finder.Getter = newGetter()
	assets, err := finder.Find()
	if err != nil {
		t.Fatalf("Find() stable: %v", err)
	}
	if len(assets) != 1 || assets[0] != "https://example.com/stable.tar.gz" {
		t.Fatalf("expected the stable asset, got %#v", assets)
	}

	// -p takes the newest match, even when it is a prerelease.
	prereleaseFinder := NewAssetFinder("acme/app", "tags/PRE:app-v", true, time.Time{})
	prereleaseFinder.Getter = newGetter()
	assets, err = prereleaseFinder.Find()
	if err != nil {
		t.Fatalf("Find() prerelease: %v", err)
	}
	if len(assets) != 1 || assets[0] != "https://example.com/rc.tar.gz" {
		t.Fatalf("expected the prerelease asset, got %#v", assets)
	}
}

func TestAssetFinderFindMatchReportsNoUpgradeForStaleMatch(t *testing.T) {
	getter := &fakeGetter{
		responses: map[string]*http.Response{
			"https://api.github.com/repos/trycua/cua/releases?page=1": jsonResponse(http.StatusOK, `[{"tag_name":"cua-driver-rs-v0.34.0","created_at":"2026-10-05T00:00:00Z","assets":[]}]`),
		},
	}
	finder := NewAssetFinder("trycua/cua", "tags/PRE:cua-driver-rs-v", false, time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC))
	finder.Getter = getter

	if _, err := finder.Find(); err != ErrNoUpgrade {
		t.Fatalf("Find() err = %v, want ErrNoUpgrade", err)
	}
}
