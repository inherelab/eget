package cli

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gookit/goutil/testutil/assert"
	"github.com/inherelab/eget/internal/install"
)

func TestGitHubSearchClientBuildsRequestURL(t *testing.T) {
	client := newGitHubSearchClient(install.Options{})
	client.get = func(rawURL string, opts install.Options) (*http.Response, error) {
		assert.Contains(t, rawURL, "https://api.github.com/search/repositories?")
		assert.Contains(t, rawURL, "q=ripgrep+language%3Arust")
		assert.Contains(t, rawURL, "per_page=5")
		assert.Contains(t, rawURL, "sort=stars")
		assert.Contains(t, rawURL, "order=desc")

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewBufferString(`{"total_count":0,"items":[]}`)),
		}, nil
	}

	result, err := client.SearchRepositories("ripgrep language:rust", 5, "stars", "desc")
	assert.Nil(t, err)
	assert.Eq(t, 0, result.TotalCount)
	assert.Len(t, result.Items, 0)
}

func TestGitHubSearchClientParsesResponse(t *testing.T) {
	client := newGitHubSearchClient(install.Options{})
	client.get = func(rawURL string, opts install.Options) (*http.Response, error) {
		body := `{"total_count":2,"items":[{"full_name":"BurntSushi/ripgrep","description":"fast search","html_url":"https://github.com/BurntSushi/ripgrep","homepage":"https://example.com","language":"Rust","stargazers_count":12,"forks_count":3,"open_issues_count":1,"updated_at":"2026-04-22T10:00:00Z","archived":false,"private":false}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}, nil
	}

	result, err := client.SearchRepositories("ripgrep", 10, "", "")
	assert.Nil(t, err)
	assert.Eq(t, 2, result.TotalCount)
	assert.Len(t, result.Items, 1)
	assert.Eq(t, "BurntSushi/ripgrep", result.Items[0].FullName)
	assert.Eq(t, "Rust", result.Items[0].Language)
	assert.Eq(t, 12, result.Items[0].StargazersCount)
	assert.Eq(t, time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC), result.Items[0].UpdatedAt)
}

func TestGitHubSearchClientReturnsErrorOnNon200(t *testing.T) {
	client := newGitHubSearchClient(install.Options{})
	client.get = func(rawURL string, opts install.Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Status:     "403 Forbidden",
			Body:       io.NopCloser(bytes.NewBufferString(`{"message":"API rate limit exceeded"}`)),
		}, nil
	}

	_, err := client.SearchRepositories("ripgrep", 10, "", "")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "search failed: 403 Forbidden"))
	assert.True(t, strings.Contains(err.Error(), `{"message":"API rate limit exceeded"}`))
}
