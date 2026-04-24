package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/inherelab/eget/internal/app"
	"github.com/inherelab/eget/internal/install"
)

type gitHubSearchClient struct {
	opts install.Options
	get  func(url string, opts install.Options) (*http.Response, error)
}

func newGitHubSearchClient(opts install.Options) *gitHubSearchClient {
	return &gitHubSearchClient{
		opts: opts,
		get:  githubAPIGetWithOptions,
	}
}

func (c *gitHubSearchClient) SearchRepositories(query string, limit int, sort, order string) (app.SearchResult, error) {
	values := url.Values{}
	values.Set("q", query)
	values.Set("per_page", strconv.Itoa(limit))
	if sort != "" {
		values.Set("sort", sort)
	}
	if order != "" {
		values.Set("order", order)
	}

	resp, err := c.get("https://api.github.com/search/repositories?"+values.Encode(), c.opts)
	if err != nil {
		return app.SearchResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return app.SearchResult{}, fmt.Errorf("search failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload struct {
		TotalCount int              `json:"total_count"`
		Items      []app.SearchRepo `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return app.SearchResult{}, err
	}

	return app.SearchResult{
		TotalCount: payload.TotalCount,
		Items:      payload.Items,
	}, nil
}
