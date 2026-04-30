package forge

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gookit/goutil/testutil/assert"
)

type fakeGetter struct {
	responses map[string]string
	statuses  map[string]int
	requests  []string
}

func (g *fakeGetter) Get(url string) (*http.Response, error) {
	g.requests = append(g.requests, url)
	status := http.StatusOK
	if g.statuses != nil && g.statuses[url] != 0 {
		status = g.statuses[url]
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(g.responses[url])),
	}, nil
}

type responseGetter struct {
	resp *http.Response
}

func (g responseGetter) Get(string) (*http.Response, error) {
	return g.resp, nil
}

func TestGitLabFinderFindsLatestReleaseAssets(t *testing.T) {
	target, err := ParseTarget("gitlab:fdroid/fdroidserver")
	assert.NoErr(t, err)

	url := "https://gitlab.com/api/v4/projects/fdroid%2Ffdroidserver/releases/permalink/latest"
	getter := &fakeGetter{responses: map[string]string{
		url: `{"tag_name":"v2.3.4","assets":{"links":[{"name":"fdroidserver-linux-amd64.tar.gz","direct_asset_url":"https://gitlab.com/fdroid/fdroidserver/-/releases/v2.3.4/downloads/fdroidserver-linux-amd64.tar.gz","url":"https://gitlab.com/fdroid/fdroidserver/-/releases/v2.3.4/downloads/metadata/fdroidserver-linux-amd64.tar.gz"},{"name":"fdroidserver.sha256","url":"https://gitlab.com/fdroid/fdroidserver/-/releases/v2.3.4/downloads/fdroidserver.sha256"}]}}`,
	}}

	assets, err := Finder{Target: target, Getter: getter}.Find()

	assert.NoErr(t, err)
	assert.Eq(t, []string{
		"https://gitlab.com/fdroid/fdroidserver/-/releases/v2.3.4/downloads/fdroidserver-linux-amd64.tar.gz",
		"https://gitlab.com/fdroid/fdroidserver/-/releases/v2.3.4/downloads/fdroidserver.sha256",
	}, assets)
	assert.Eq(t, []string{url}, getter.requests)
}

func TestGitLabFinderFindsSpecificTag(t *testing.T) {
	target, err := ParseTarget("gitlab:gitlab.example.com/group/subgroup/project")
	assert.NoErr(t, err)
	url := "https://gitlab.example.com/api/v4/projects/group%2Fsubgroup%2Fproject/releases/v1.0.0"
	getter := &fakeGetter{responses: map[string]string{
		url: `{"tag_name":"v1.0.0","assets":{"links":[{"name":"project.exe","url":"https://gitlab.example.com/group/subgroup/project/-/releases/v1.0.0/downloads/project.exe"}]}}`,
	}}

	assets, err := Finder{Target: target, Tag: "v1.0.0", Getter: getter}.Find()

	assert.NoErr(t, err)
	assert.Eq(t, []string{"https://gitlab.example.com/group/subgroup/project/-/releases/v1.0.0/downloads/project.exe"}, assets)
	assert.Eq(t, []string{url}, getter.requests)
}

func TestGitLabFinderRejectsEmptyAssets(t *testing.T) {
	target, err := ParseTarget("gitlab:fdroid/fdroidserver")
	assert.NoErr(t, err)
	url := "https://gitlab.com/api/v4/projects/fdroid%2Ffdroidserver/releases/permalink/latest"
	getter := &fakeGetter{responses: map[string]string{url: `{"tag_name":"v2.3.4","assets":{"links":[]}}`}}

	_, err = Finder{Target: target, Getter: getter}.Find()

	if err == nil || !strings.Contains(err.Error(), "gitlab release assets not found") {
		t.Fatalf("expected empty assets error, got %v", err)
	}
}

func TestGitLabFinderReportsHTTPStatus(t *testing.T) {
	target, err := ParseTarget("gitlab:fdroid/fdroidserver")
	assert.NoErr(t, err)
	url := "https://gitlab.com/api/v4/projects/fdroid%2Ffdroidserver/releases/permalink/latest"
	getter := &fakeGetter{
		responses: map[string]string{url: `{"message":"404 Not found"}`},
		statuses:  map[string]int{url: http.StatusNotFound},
	}

	_, err = Finder{Target: target, Getter: getter}.Find()

	if err == nil || !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), url) {
		t.Fatalf("expected status and URL error, got %v", err)
	}
}

func TestGitLabFinderReportsNilResponse(t *testing.T) {
	target, err := ParseTarget("gitlab:fdroid/fdroidserver")
	assert.NoErr(t, err)

	_, err = Finder{Target: target, Getter: responseGetter{}}.Find()

	if err == nil || !strings.Contains(err.Error(), "gitlab release response is nil") {
		t.Fatalf("expected nil response error, got %v", err)
	}
}

func TestGitLabFinderReportsNilResponseBody(t *testing.T) {
	target, err := ParseTarget("gitlab:fdroid/fdroidserver")
	assert.NoErr(t, err)

	_, err = Finder{
		Target: target,
		Getter: responseGetter{resp: &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
		}},
	}.Find()

	if err == nil || !strings.Contains(err.Error(), "gitlab release response body is nil") {
		t.Fatalf("expected nil response body error, got %v", err)
	}
}

func TestGitLabLatestVersion(t *testing.T) {
	target, err := ParseTarget("gitlab:fdroid/fdroidserver")
	assert.NoErr(t, err)
	url := "https://gitlab.com/api/v4/projects/fdroid%2Ffdroidserver/releases/permalink/latest"
	getter := &fakeGetter{responses: map[string]string{
		url: `{"tag_name":"v2.3.4","assets":{"links":[{"name":"tool.tar.gz","url":"https://gitlab.com/tool.tar.gz"}]}}`,
	}}

	info, err := LatestVersion(target, getter)

	assert.NoErr(t, err)
	assert.Eq(t, "v2.3.4", info.Tag)
}

func TestLatestVersionRequiresGetter(t *testing.T) {
	target, err := ParseTarget("gitlab:fdroid/fdroidserver")
	assert.NoErr(t, err)

	_, err = LatestVersion(target, nil)

	if err == nil || !strings.Contains(err.Error(), "forge HTTP getter is required") {
		t.Fatalf("expected getter required error, got %v", err)
	}
}
