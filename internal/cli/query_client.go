package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/inherelab/eget/internal/app"
	"github.com/inherelab/eget/internal/install"
)

type gitHubQueryClient struct {
	opts install.Options
	get  func(url string, opts install.Options) (*http.Response, error)
}

func newGitHubQueryClient(opts install.Options) *gitHubQueryClient {
	return &gitHubQueryClient{
		opts: opts,
		get:  githubAPIGetWithOptions,
	}
}

func (c *gitHubQueryClient) RepoInfo(repo string) (app.QueryRepoInfo, error) {
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
	if err := c.fetchJSON(fmt.Sprintf("https://api.github.com/repos/%s", repo), &payload); err != nil {
		return app.QueryRepoInfo{}, err
	}
	return app.QueryRepoInfo{
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

func (c *gitHubQueryClient) LatestRelease(repo string, includePrerelease bool) (app.QueryRelease, error) {
	if includePrerelease {
		releases, err := c.ListReleases(repo, 1, true)
		if err != nil {
			return app.QueryRelease{}, err
		}
		if len(releases) == 0 {
			return app.QueryRelease{}, fmt.Errorf("no releases found")
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
	if err := c.fetchJSON(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo), &payload); err != nil {
		return app.QueryRelease{}, err
	}
	return app.QueryRelease{
		Tag:         payload.Tag,
		Name:        payload.Name,
		Prerelease:  payload.Prerelease,
		AssetsCount: len(payload.Assets),
		PublishedAt: parseRFC3339Time(payload.PublishedAt),
	}, nil
}

func (c *gitHubQueryClient) ListReleases(repo string, limit int, includePrerelease bool) ([]app.QueryRelease, error) {
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
	if err := c.fetchJSON(fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=%d", repo, limit), &payload); err != nil {
		return nil, err
	}
	items := make([]app.QueryRelease, 0, len(payload))
	for _, item := range payload {
		if !includePrerelease && item.Prerelease {
			continue
		}
		items = append(items, app.QueryRelease{
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

func (c *gitHubQueryClient) ReleaseAssets(repo, tag string) ([]app.QueryAsset, error) {
	var payload struct {
		Assets []struct {
			Name          string `json:"name"`
			Size          int64  `json:"size"`
			DownloadCount int    `json:"download_count"`
			UpdatedAt     string `json:"updated_at"`
			URL           string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := c.fetchJSON(fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repo, tag), &payload); err != nil {
		return nil, err
	}
	items := make([]app.QueryAsset, 0, len(payload.Assets))
	for _, item := range payload.Assets {
		items = append(items, app.QueryAsset{
			Name:          item.Name,
			Size:          item.Size,
			DownloadCount: item.DownloadCount,
			UpdatedAt:     parseRFC3339Time(item.UpdatedAt),
			URL:           item.URL,
		})
	}
	return items, nil
}

func (c *gitHubQueryClient) fetchJSON(url string, target any) error {
	resp, err := c.get(url, c.opts)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("query failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
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
