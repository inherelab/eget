package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gookit/goutil/testutil/assert"
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
		body := `{"assets":[{"name":"tool-linux-amd64.tar.gz","size":12,"download_count":3,"updated_at":"2026-04-22T10:00:00Z","browser_download_url":"https://example.com/tool"}]}`
		return jsonResponse(http.StatusOK, "200 OK", body), nil
	})

	got, err := client.ReleaseAssets("owner/repo", "v1.2.3")
	if err != nil {
		t.Fatalf("ReleaseAssets(): %v", err)
	}
	if len(got) != 1 || got[0].Name != "tool-linux-amd64.tar.gz" {
		t.Fatalf("unexpected assets: %#v", got)
	}
}

func TestGitHubClientLatestReleaseInfo(t *testing.T) {
	var requestedURL string
	client := NewGitHubClientWithGetter(Options{}, func(rawURL string, opts Options) (*http.Response, error) {
		requestedURL = rawURL
		payload, err := json.Marshal(map[string]any{
			"tag_name":   "v0.3.6",
			"created_at": "2026-04-20T14:10:17Z",
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

	tag, createdAt, err := client.LatestReleaseInfo("gookit/gitw")
	if err != nil {
		t.Fatalf("LatestReleaseInfo(): %v", err)
	}
	if requestedURL != "https://api.github.com/repos/gookit/gitw/releases/latest" {
		t.Fatalf("unexpected request url: %s", requestedURL)
	}
	if tag != "v0.3.6" {
		t.Fatalf("expected tag v0.3.6, got %q", tag)
	}
	wantTime := time.Date(2026, 4, 20, 14, 10, 17, 0, time.UTC)
	if !createdAt.Equal(wantTime) {
		t.Fatalf("expected created_at %s, got %s", wantTime, createdAt)
	}
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

func jsonResponse(code int, status, body string) *http.Response {
	return &http.Response{
		StatusCode: code,
		Status:     status,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}
