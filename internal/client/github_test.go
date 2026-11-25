package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
)

func TestGitHubClientSearchBuildsRequestURL(t *testing.T) {
	client := NewGitHubClientWithGetter(Options{}, func(rawURL string, opts Options) (*http.Response, error) {
		assert.Contains(t, rawURL, "https://api.github.com/search/repositories?")
		assert.Contains(t, rawURL, "q=ripgrep+language%3Arust")
		assert.Contains(t, rawURL, "per_page=5")
		assert.Contains(t, rawURL, "sort=stars")
		assert.Contains(t, rawURL, "order=desc")

		return jsonResponse(http.StatusOK, "200 OK", `{"total_count":0,"items":[]}`), nil
	})

	result, err := client.SearchRepositories("ripgrep language:rust", 5, "stars", "desc")
	assert.Nil(t, err)
	assert.Eq(t, 0, result.TotalCount)
	assert.Len(t, result.Items, 0)
}

func TestGitHubClientSearchParsesResponse(t *testing.T) {
	client := NewGitHubClientWithGetter(Options{}, func(rawURL string, opts Options) (*http.Response, error) {
		body := `{"total_count":2,"items":[{"full_name":"BurntSushi/ripgrep","description":"fast search","html_url":"https://github.com/BurntSushi/ripgrep","homepage":"https://example.com","language":"Rust","stargazers_count":12,"forks_count":3,"open_issues_count":1,"updated_at":"2026-04-22T10:00:00Z","archived":false,"private":false}]}`
		return jsonResponse(http.StatusOK, "200 OK", body), nil
	})

	result, err := client.SearchRepositories("ripgrep", 10, "", "")
	assert.Nil(t, err)
	assert.Eq(t, 2, result.TotalCount)
	assert.Len(t, result.Items, 1)
	assert.Eq(t, "BurntSushi/ripgrep", result.Items[0].FullName)
	assert.Eq(t, "Rust", result.Items[0].Language)
	assert.Eq(t, 12, result.Items[0].StargazersCount)
	assert.Eq(t, time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC), result.Items[0].UpdatedAt)
}

func TestGitHubClientSearchReturnsErrorOnNon200(t *testing.T) {
	client := NewGitHubClientWithGetter(Options{}, func(rawURL string, opts Options) (*http.Response, error) {
		return jsonResponse(http.StatusForbidden, "403 Forbidden", `{"message":"API rate limit exceeded"}`), nil
	})

	_, err := client.SearchRepositories("ripgrep", 10, "", "")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "search failed: 403 Forbidden"))
	assert.True(t, strings.Contains(err.Error(), `{"message":"API rate limit exceeded"}`))
}

func TestGitHubClientLatestRelease(t *testing.T) {
	client := NewGitHubClientWithGetter(Options{}, func(rawURL string, opts Options) (*http.Response, error) {
		body := `{"tag_name":"v1.2.3","name":"v1.2.3","prerelease":false,"published_at":"2026-04-22T10:00:00Z","assets":[{},{}]}`
		return jsonResponse(http.StatusOK, "200 OK", body), nil
	})

	got, err := client.LatestRelease("owner/repo", false)
	if err != nil {
		t.Fatalf("LatestRelease(): %v", err)
	}
	if got.Tag != "v1.2.3" || got.AssetsCount != 2 {
		t.Fatalf("unexpected latest release: %#v", got)
	}
}

func TestGitHubClientReleaseAssets(t *testing.T) {
	client := NewGitHubClientWithGetter(Options{}, func(rawURL string, opts Options) (*http.Response, error) {
		body := `{"assets":[{"id":100,"name":"tool-linux-amd64.tar.gz","size":12,"download_count":3,"updated_at":"2026-04-22T10:00:00Z","digest":"sha256:abc","browser_download_url":"https://example.com/tool"}]}`
		return jsonResponse(http.StatusOK, "200 OK", body), nil
	})

	got, err := client.ReleaseAssets("owner/repo", "v1.2.3")
	if err != nil {
		t.Fatalf("ReleaseAssets(): %v", err)
	}
	if len(got) != 1 || got[0].Name != "tool-linux-amd64.tar.gz" {
		t.Fatalf("unexpected assets: %#v", got)
	}
	assert.Eq(t, int64(100), got[0].ID)
	assert.Eq(t, int64(12), got[0].Size)
	assert.Eq(t, "sha256:abc", got[0].Digest)
}

func TestGitHubClientLatestReleaseInfo(t *testing.T) {
	var requestedURL string
	client := NewGitHubClientWithGetter(Options{}, func(rawURL string, opts Options) (*http.Response, error) {
		requestedURL = rawURL
		payload, err := json.Marshal(map[string]any{
			"tag_name":     "v0.3.6",
			"created_at":   "2026-04-20T14:10:17Z",
			"published_at": "2026-04-21T14:10:17Z",
		})
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewReader(payload)),
			Header:     make(http.Header),
		}, nil
	})

	tag, publishedAt, err := client.LatestReleaseInfo("gookit/gitw")
	if err != nil {
		t.Fatalf("LatestReleaseInfo(): %v", err)
	}
	if requestedURL != "https://api.github.com/repos/gookit/gitw/releases/latest" {
		t.Fatalf("unexpected request url: %s", requestedURL)
	}
	if tag != "v0.3.6" {
		t.Fatalf("expected tag v0.3.6, got %q", tag)
	}
	wantTime := time.Date(2026, 4, 21, 14, 10, 17, 0, time.UTC)
	if !publishedAt.Equal(wantTime) {
		t.Fatalf("expected published_at %s, got %s", wantTime, publishedAt)
	}
}

func TestGitHubClientLatestReleaseInfoCanIncludePrerelease(t *testing.T) {
	var requestedURL string
	client := NewGitHubClientWithGetter(Options{}, func(rawURL string, opts Options) (*http.Response, error) {
		requestedURL = rawURL
		payload := `[{"tag_name":"v0.12.0-beta","name":"beta","prerelease":true,"published_at":"2026-04-21T14:10:17Z","assets":[]}]`
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(payload)),
			Header:     make(http.Header),
		}, nil
	})

	tag, publishedAt, err := client.LatestReleaseInfo("pkgforge/aeris", true)

	assert.NoErr(t, err)
	assert.Eq(t, "https://api.github.com/repos/pkgforge/aeris/releases?per_page=30", requestedURL)
	assert.Eq(t, "v0.12.0-beta", tag)
	assert.Eq(t, time.Date(2026, 4, 21, 14, 10, 17, 0, time.UTC), publishedAt)
}

// A release drafted earlier but published later must win over an older publish
// that the API returned first.
func TestGitHubClientLatestReleaseInfoPicksNewestPublished(t *testing.T) {
	client := NewGitHubClientWithGetter(Options{}, func(rawURL string, opts Options) (*http.Response, error) {
		payload := `[
			{"tag_name":"v1.9.0","prerelease":true,"published_at":"2026-11-21T17:31:33Z","assets":[]},
			{"tag_name":"v1.8.0","prerelease":true,"published_at":"2026-11-21T14:33:13Z","assets":[]},
			{"tag_name":"v1.10.0-rc.1","prerelease":true,"published_at":"2026-11-22T08:00:00Z","assets":[]}
		]`
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(payload)),
			Header:     make(http.Header),
		}, nil
	})

	tag, publishedAt, err := client.LatestReleaseInfo("colitu/linux", true)

	assert.NoErr(t, err)
	assert.Eq(t, "v1.10.0-rc.1", tag)
	assert.Eq(t, time.Date(2026, 11, 22, 8, 0, 0, 0, time.UTC), publishedAt)
}

// A repository that publishes prereleases only has no latest release: the API
// answers 404 and eget has to fall back to the newest published release instead
// of failing the install or the update check.
func TestGitHubClientLatestReleaseInfoFallsBackToPrereleaseOnlyRepo(t *testing.T) {
	var requested []string
	client := NewGitHubClientWithGetter(Options{}, func(rawURL string, opts Options) (*http.Response, error) {
		requested = append(requested, rawURL)
		switch {
		case strings.HasSuffix(rawURL, "/releases/latest"):
			return jsonResponse(http.StatusNotFound, "404 Not Found", `{"message":"Not Found"}`), nil
		default:
			body := `[
				{"tag_name":"v1.9.0","prerelease":true,"published_at":"2026-11-21T17:31:33Z"},
				{"tag_name":"v1.8.0","prerelease":true,"published_at":"2026-11-21T14:33:13Z"}
			]`
			return jsonResponse(http.StatusOK, "200 OK", body), nil
		}
	})

	tag, publishedAt, err := client.LatestReleaseInfo("colitu/linux")

	assert.NoErr(t, err)
	assert.Eq(t, "v1.9.0", tag)
	assert.Eq(t, time.Date(2026, 11, 21, 17, 31, 33, 0, time.UTC), publishedAt)
	assert.Eq(t, 2, len(requested))
	assert.Eq(t, "https://api.github.com/repos/colitu/linux/releases/latest", requested[0])
	assert.Eq(t, "https://api.github.com/repos/colitu/linux/releases?per_page=30", requested[1])
}

func TestGitHubClientLatestReleaseFallsBackToNewestPublishedPrerelease(t *testing.T) {
	client := NewGitHubClientWithGetter(Options{}, func(rawURL string, opts Options) (*http.Response, error) {
		if strings.HasSuffix(rawURL, "/releases/latest") {
			return jsonResponse(http.StatusNotFound, "404 Not Found", `{"message":"Not Found"}`), nil
		}
		body := `[
			{"tag_name":"v1.9.0","prerelease":true,"published_at":"2026-11-21T17:31:33Z","assets":[{}]},
			{"tag_name":"v1.10.0-rc.1","prerelease":true,"published_at":"2026-11-22T08:00:00Z","assets":[]}
		]`
		return jsonResponse(http.StatusOK, "200 OK", body), nil
	})

	release, err := client.LatestRelease("colitu/linux", false)

	assert.NoErr(t, err)
	assert.Eq(t, "v1.10.0-rc.1", release.Tag)
	assert.True(t, release.Prerelease)
}

// The 404 fallback must not hide real API failures such as rate limiting.
func TestGitHubClientLatestReleaseInfoKeepsNonNotFoundError(t *testing.T) {
	requested := 0
	client := NewGitHubClientWithGetter(Options{}, func(rawURL string, opts Options) (*http.Response, error) {
		requested++
		return jsonResponse(http.StatusForbidden, "403 Forbidden", `{"message":"API rate limit exceeded"}`), nil
	})

	_, _, err := client.LatestReleaseInfo("colitu/linux")

	assert.ErrMsgContains(t, err, "latest tag check failed: 403 Forbidden")
	assert.Eq(t, 1, requested)
}

func TestGitHubClientReleaseInfoUsesTagEndpoint(t *testing.T) {
	var requestedURL string
	client := NewGitHubClientWithGetter(Options{}, func(rawURL string, opts Options) (*http.Response, error) {
		requestedURL = rawURL
		return jsonResponse(http.StatusOK, "200 OK", `{"tag_name":"nightly","published_at":"2026-04-21T14:10:17Z"}`), nil
	})

	tag, publishedAt, err := client.ReleaseInfo("pkgforge/aeris", "nightly")

	assert.NoErr(t, err)
	assert.Eq(t, "https://api.github.com/repos/pkgforge/aeris/releases/tags/nightly", requestedURL)
	assert.Eq(t, "nightly", tag)
	assert.Eq(t, time.Date(2026, 4, 21, 14, 10, 17, 0, time.UTC), publishedAt)
}

func TestGitHubClientGetUsesSharedGetter(t *testing.T) {
	var requestedURL string
	client := NewGitHubClientWithGetter(Options{}, func(rawURL string, opts Options) (*http.Response, error) {
		requestedURL = rawURL
		return jsonResponse(http.StatusOK, "200 OK", `{}`), nil
	})

	resp, err := client.Get("https://api.github.com/repos/owner/repo/releases/latest")
	if err != nil {
		t.Fatalf("Get(): %v", err)
	}
	defer resp.Body.Close()
	if requestedURL != "https://api.github.com/repos/owner/repo/releases/latest" {
		t.Fatalf("unexpected request url: %s", requestedURL)
	}
}
