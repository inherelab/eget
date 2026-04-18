package github

import (
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
			"https://api.github.com/repos/inhere/markview/releases/latest": jsonResponse(http.StatusOK, `{"assets":[{"browser_download_url":"https://example.com/tool.tar.gz"}],"created_at":"2026-04-18T00:00:00Z"}`),
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
}

func TestAssetFinderFindMatchFallback(t *testing.T) {
	getter := &fakeGetter{
		responses: map[string]*http.Response{
			"https://api.github.com/repos/inhere/markview/releases/tags/v1.2.3": jsonResponse(http.StatusNotFound, `{"message":"not found"}`),
			"https://api.github.com/repos/inhere/markview/releases?page=1":   jsonResponse(http.StatusOK, `[{"tag_name":"release-v1.2.3","created_at":"2026-04-18T00:00:00Z","assets":[{"browser_download_url":"https://example.com/match.tar.gz"}]}]`),
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
			"https://api.github.com/repos/inhere/markview/releases":        jsonResponse(http.StatusOK, `[{"tag_name":"v2.0.0-rc1"}]`),
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
