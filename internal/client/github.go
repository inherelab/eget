package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type HTTPGetFunc func(url string, opts Options) (*http.Response, error)

type RepoInfo struct {
	Repo          string    `json:"repo"`
	Description   string    `json:"description,omitempty"`
	Homepage      string    `json:"homepage,omitempty"`
	DefaultBranch string    `json:"default_branch,omitempty"`
	Stars         int       `json:"stars,omitempty"`
	Forks         int       `json:"forks,omitempty"`
	OpenIssues    int       `json:"open_issues,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

type Release struct {
	Tag         string    `json:"tag"`
	Name        string    `json:"name,omitempty"`
	PublishedAt time.Time `json:"published_at,omitempty"`
	Prerelease  bool      `json:"prerelease,omitempty"`
	AssetsCount int       `json:"assets_count,omitempty"`
}

type Asset struct {
	Name          string    `json:"name"`
	Size          int64     `json:"size,omitempty"`
	DownloadCount int       `json:"download_count,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
	URL           string    `json:"url,omitempty"`
}

type SearchRepo struct {
	FullName        string    `json:"full_name,omitempty"`
	Description     string    `json:"description,omitempty"`
	HTMLURL         string    `json:"html_url,omitempty"`
	Homepage        string    `json:"homepage,omitempty"`
	Language        string    `json:"language,omitempty"`
	StargazersCount int       `json:"stargazers_count,omitempty"`
	ForksCount      int       `json:"forks_count,omitempty"`
	OpenIssuesCount int       `json:"open_issues_count,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
	Archived        bool      `json:"archived,omitempty"`
	Private         bool      `json:"private,omitempty"`
}

type SearchResult struct {
	Query      string       `json:"query"`
	TotalCount int          `json:"total_count"`
	Items      []SearchRepo `json:"items,omitempty"`
}

type GitHubClient struct {
	opts Options
	get  HTTPGetFunc
}

func NewGitHubClient(opts Options) *GitHubClient {
	return &GitHubClient{
		opts: opts,
		get:  GetWithOptions,
	}
}

func NewGitHubClientWithGetter(opts Options, get HTTPGetFunc) *GitHubClient {
	client := NewGitHubClient(opts)
	if get != nil {
		client.get = get
	}
	return client
}

func (c *GitHubClient) Get(rawURL string) (*http.Response, error) {
	return c.get(rawURL, c.opts)
}

func (c *GitHubClient) RepoInfo(repo string) (RepoInfo, error) {
	var payload struct {
		FullName      string    `json:"full_name"`
		Description   string    `json:"description"`
		Homepage      string    `json:"homepage"`
		DefaultBranch string    `json:"default_branch"`
		Stars         int       `json:"stargazers_count"`
		Forks         int       `json:"forks_count"`
		OpenIssues    int       `json:"open_issues_count"`
		UpdatedAt     time.Time `json:"updated_at"`
	}
	if err := c.fetchJSON(fmt.Sprintf("https://api.github.com/repos/%s", repo), "query", &payload); err != nil {
		return RepoInfo{}, err
	}
	return RepoInfo{
		Repo:          payload.FullName,
		Description:   payload.Description,
		Homepage:      payload.Homepage,
		DefaultBranch: payload.DefaultBranch,
		Stars:         payload.Stars,
		Forks:         payload.Forks,
		OpenIssues:    payload.OpenIssues,
		UpdatedAt:     payload.UpdatedAt,
	}, nil
}

func (c *GitHubClient) LatestRelease(repo string, includePrerelease bool) (Release, error) {
	if includePrerelease {
		releases, err := c.ListReleases(repo, 1, true)
		if err != nil {
			return Release{}, err
		}
		if len(releases) == 0 {
			return Release{}, fmt.Errorf("no releases found")
		}
		return releases[0], nil
	}

	var payload struct {
		Tag         string `json:"tag_name"`
		Name        string `json:"name"`
		Prerelease  bool   `json:"prerelease"`
		Assets      []any  `json:"assets"`
		PublishedAt string `json:"published_at"`
	}
	if err := c.fetchJSON(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo), "query", &payload); err != nil {
		return Release{}, err
	}
	return Release{
		Tag:         payload.Tag,
		Name:        payload.Name,
		Prerelease:  payload.Prerelease,
		AssetsCount: len(payload.Assets),
		PublishedAt: parseRFC3339Time(payload.PublishedAt),
	}, nil
}

func (c *GitHubClient) LatestReleaseInfo(repo string) (string, time.Time, error) {
	var payload struct {
		Tag       string    `json:"tag_name"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := c.fetchJSON(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo), "latest tag check", &payload); err != nil {
		return "", time.Time{}, err
	}
	if payload.Tag == "" {
		return "", time.Time{}, fmt.Errorf("latest tag is empty")
	}
	return payload.Tag, payload.CreatedAt, nil
}

func (c *GitHubClient) ListReleases(repo string, limit int, includePrerelease bool) ([]Release, error) {
	if limit <= 0 {
		limit = 10
	}
	var payload []struct {
		Tag         string `json:"tag_name"`
		Name        string `json:"name"`
		Prerelease  bool   `json:"prerelease"`
		Assets      []any  `json:"assets"`
		PublishedAt string `json:"published_at"`
	}
	if err := c.fetchJSON(fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=%d", repo, limit), "query", &payload); err != nil {
		return nil, err
	}
	items := make([]Release, 0, len(payload))
	for _, item := range payload {
		if !includePrerelease && item.Prerelease {
			continue
		}
		items = append(items, Release{
			Tag:         item.Tag,
			Name:        item.Name,
			Prerelease:  item.Prerelease,
			AssetsCount: len(item.Assets),
			PublishedAt: parseRFC3339Time(item.PublishedAt),
		})
		if len(items) == limit {
			break
		}
	}
	return items, nil
}

func (c *GitHubClient) ReleaseAssets(repo, tag string) ([]Asset, error) {
	var payload struct {
		Assets []struct {
			Name          string `json:"name"`
			Size          int64  `json:"size"`
			DownloadCount int    `json:"download_count"`
			UpdatedAt     string `json:"updated_at"`
			URL           string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := c.fetchJSON(fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repo, tag), "query", &payload); err != nil {
		return nil, err
	}
	items := make([]Asset, 0, len(payload.Assets))
	for _, item := range payload.Assets {
		items = append(items, Asset{
			Name:          item.Name,
			Size:          item.Size,
			DownloadCount: item.DownloadCount,
			UpdatedAt:     parseRFC3339Time(item.UpdatedAt),
			URL:           item.URL,
		})
	}
	return items, nil
}

func (c *GitHubClient) SearchRepositories(query string, limit int, sort, order string) (SearchResult, error) {
	values := url.Values{}
	values.Set("q", query)
	values.Set("per_page", strconv.Itoa(limit))
	if sort != "" {
		values.Set("sort", sort)
	}
	if order != "" {
		values.Set("order", order)
	}

	var payload struct {
		TotalCount int          `json:"total_count"`
		Items      []SearchRepo `json:"items"`
	}
	if err := c.fetchJSON("https://api.github.com/search/repositories?"+values.Encode(), "search", &payload); err != nil {
		return SearchResult{}, err
	}
	return SearchResult{
		TotalCount: payload.TotalCount,
		Items:      payload.Items,
	}, nil
}

func (c *GitHubClient) fetchJSON(rawURL, action string, target any) error {
	resp, err := c.Get(rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s failed: %s: %s", action, resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func parseRFC3339Time(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
