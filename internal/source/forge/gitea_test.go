package forge

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gookit/goutil/testutil/assert"
)

func TestGiteaFinderFindsLatestReleaseAssets(t *testing.T) {
	target, err := ParseTarget("gitea:codeberg.org/forgejo/forgejo")
	assert.NoErr(t, err)

	url := "https://codeberg.org/api/v1/repos/forgejo/forgejo/releases/latest"
	getter := &fakeGetter{responses: map[string]string{
		url: `{"tag_name":"v9.0.0","assets":[{"name":"forgejo-linux-amd64","browser_download_url":"https://codeberg.org/forgejo/forgejo/releases/download/v9.0.0/forgejo-linux-amd64"},{"name":"forgejo.sha256","browser_download_url":"https://codeberg.org/forgejo/forgejo/releases/download/v9.0.0/forgejo.sha256"}]}`,
	}}

	assets, err := Finder{Target: target, Getter: getter}.Find()

	assert.NoErr(t, err)
	assert.Eq(t, []string{
		"https://codeberg.org/forgejo/forgejo/releases/download/v9.0.0/forgejo-linux-amd64",
		"https://codeberg.org/forgejo/forgejo/releases/download/v9.0.0/forgejo.sha256",
	}, assets)
	assert.Eq(t, []string{url}, getter.requests)
}

func TestForgejoFinderFindsSpecificTag(t *testing.T) {
	target, err := ParseTarget("forgejo:codeberg.org/forgejo/forgejo")
	assert.NoErr(t, err)

	url := "https://codeberg.org/api/v1/repos/forgejo/forgejo/releases/tags/v9.0.0"
	getter := &fakeGetter{responses: map[string]string{
		url: `{"tag_name":"v9.0.0","assets":[{"name":"forgejo.exe","browser_download_url":"https://codeberg.org/forgejo/forgejo/releases/download/v9.0.0/forgejo.exe"}]}`,
	}}

	assets, err := Finder{Target: target, Tag: "v9.0.0", Getter: getter}.Find()

	assert.NoErr(t, err)
	assert.Eq(t, []string{"https://codeberg.org/forgejo/forgejo/releases/download/v9.0.0/forgejo.exe"}, assets)
	assert.Eq(t, []string{url}, getter.requests)
}

func TestGiteaFinderRejectsEmptyAssets(t *testing.T) {
	target, err := ParseTarget("gitea:codeberg.org/forgejo/forgejo")
	assert.NoErr(t, err)

	url := "https://codeberg.org/api/v1/repos/forgejo/forgejo/releases/latest"
	getter := &fakeGetter{responses: map[string]string{url: `{"tag_name":"v9.0.0","assets":[]}`}}

	_, err = Finder{Target: target, Getter: getter}.Find()

	if err == nil || !strings.Contains(err.Error(), "gitea release assets not found") {
		t.Fatalf("expected empty assets error, got %v", err)
	}
}

func TestGiteaFinderReportsHTTPStatus(t *testing.T) {
	target, err := ParseTarget("gitea:codeberg.org/forgejo/forgejo")
	assert.NoErr(t, err)

	url := "https://codeberg.org/api/v1/repos/forgejo/forgejo/releases/latest"
	getter := &fakeGetter{
		responses: map[string]string{url: `{"message":"Not Found"}`},
		statuses:  map[string]int{url: http.StatusNotFound},
	}

	_, err = Finder{Target: target, Getter: getter}.Find()

	if err == nil || !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), url) {
		t.Fatalf("expected status and URL error, got %v", err)
	}
}

func TestGiteaLatestVersion(t *testing.T) {
	target, err := ParseTarget("gitea:codeberg.org/forgejo/forgejo")
	assert.NoErr(t, err)

	url := "https://codeberg.org/api/v1/repos/forgejo/forgejo/releases/latest"
	getter := &fakeGetter{responses: map[string]string{
		url: `{"tag_name":"v9.0.0","assets":[{"name":"forgejo-linux-amd64","browser_download_url":"https://codeberg.org/forgejo/forgejo/releases/download/v9.0.0/forgejo-linux-amd64"}]}`,
	}}

	info, err := LatestVersion(target, getter)

	assert.NoErr(t, err)
	assert.Eq(t, "v9.0.0", info.Tag)
}
