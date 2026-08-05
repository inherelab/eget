package app

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	sourcesf "github.com/inherelab/eget/internal/source/sourceforge"
)

type fakeQueryClient struct {
	repoInfo    QueryRepoInfo
	releases    []QueryRelease
	assets      []QueryAsset
	repoCalls   int
	latestCalls int
	listCalls   int
	assetCalls  int
	lastRepo    string
	lastTag     string
	lastLimit   int
}

func (f *fakeQueryClient) RepoInfo(repo string) (QueryRepoInfo, error) {
	f.repoCalls++
	f.lastRepo = repo
	return f.repoInfo, nil
}

func (f *fakeQueryClient) LatestRelease(repo string, includePrerelease bool) (QueryRelease, error) {
	f.latestCalls++
	f.lastRepo = repo
	return f.releases[0], nil
}

func (f *fakeQueryClient) ListReleases(repo string, limit int, includePrerelease bool) ([]QueryRelease, error) {
	f.listCalls++
	f.lastRepo = repo
	f.lastLimit = limit
	return f.releases, nil
}

func (f *fakeQueryClient) ReleaseAssets(repo, tag string) ([]QueryAsset, error) {
	f.assetCalls++
	f.lastRepo = repo
	f.lastTag = tag
	return f.assets, nil
}

func TestQueryServiceSourceForgeLatest(t *testing.T) {
	svc := QueryService{
		SourceForgeLatest: func(project, sourcePath string) (sourcesf.LatestInfo, error) {
			assert.Eq(t, "project", project)
			assert.Eq(t, "files", sourcePath)
			return sourcesf.LatestInfo{
				Version:     "1.2.3",
				Path:        "files/1.2.3",
				PublishedAt: time.Date(2026, 2, 3, 9, 21, 40, 0, time.UTC),
				AssetsCount: 2,
			}, nil
		},
	}

	result, err := svc.Query(QueryOptions{Repo: "sourceforge:project/files"})
	if err != nil {
		t.Fatalf("Query(): %v", err)
	}

	assert.Eq(t, "latest", result.Action)
	assert.Eq(t, "sourceforge:project/files", result.Repo)
	if result.Latest == nil {
		t.Fatal("expected latest release result")
	}
	assert.Eq(t, "1.2.3", result.Latest.Tag)
	assert.Eq(t, "1.2.3", result.Latest.Name)
	assert.Eq(t, time.Date(2026, 2, 3, 9, 21, 40, 0, time.UTC), result.Latest.PublishedAt)
	assert.Eq(t, 2, result.Latest.AssetsCount)
}

func TestQueryServiceSourceForgeAliasNormalizesRepo(t *testing.T) {
	svc := QueryService{
		SourceForgeLatest: func(project, sourcePath string) (sourcesf.LatestInfo, error) {
			assert.Eq(t, "project", project)
			assert.Eq(t, "files", sourcePath)
			return sourcesf.LatestInfo{Version: "1.2.3"}, nil
		},
	}

	result, err := svc.Query(QueryOptions{Repo: "sf:project/files"})
	if err != nil {
		t.Fatalf("Query(): %v", err)
	}

	assert.Eq(t, "sourceforge:project/files", result.Repo)
}

func TestQueryServiceSourceForgeProjectURLNormalizesRepo(t *testing.T) {
	svc := QueryService{
		SourceForgeLatest: func(project, sourcePath string) (sourcesf.LatestInfo, error) {
			assert.Eq(t, "victoria-ssd-hdd", project)
			assert.Eq(t, "", sourcePath)
			return sourcesf.LatestInfo{Tag: "Victoria537.zip", Version: "Victoria537"}, nil
		},
	}

	result, err := svc.Query(QueryOptions{Repo: "https://sourceforge.net/projects/victoria-ssd-hdd"})
	if err != nil {
		t.Fatalf("Query(): %v", err)
	}

	assert.Eq(t, "sourceforge:victoria-ssd-hdd", result.Repo)
	assert.Eq(t, "Victoria537.zip", result.Latest.Tag)
	assert.Eq(t, "Victoria537", result.Latest.Name)
}

func TestQueryServiceSourceForgeAssets(t *testing.T) {
	svc := QueryService{
		SourceForgeAssets: func(project, sourcePath, tag string) ([]string, error) {
			assert.Eq(t, "project", project)
			assert.Eq(t, "files", sourcePath)
			assert.Eq(t, "1.2.3", tag)
			return []string{
				"https://downloads.sourceforge.net/project/project/files/1.2.3/tool-linux.tar.gz",
				"https://downloads.sourceforge.net/project/project/files/1.2.3/tool%20windows.zip",
			}, nil
		},
	}

	result, err := svc.Query(QueryOptions{Repo: "sourceforge:project/files", Action: "assets", Tag: "1.2.3"})
	if err != nil {
		t.Fatalf("Query(): %v", err)
	}

	assert.Eq(t, "assets", result.Action)
	assert.Eq(t, "1.2.3", result.Tag)
	assert.Eq(t, 2, len(result.Assets))
	assert.Eq(t, "tool-linux.tar.gz", result.Assets[0].Name)
	assert.Eq(t, "tool windows.zip", result.Assets[1].Name)
	assert.Eq(t, "https://downloads.sourceforge.net/project/project/files/1.2.3/tool-linux.tar.gz", result.Assets[0].URL)
}

func TestQueryServiceSourceForgeReleases(t *testing.T) {
	svc := QueryService{
		SourceForgeReleases: func(project, sourcePath string, limit int, includePrerelease bool) ([]sourcesf.LatestInfo, error) {
			assert.Eq(t, "project", project)
			assert.Eq(t, "files", sourcePath)
			assert.Eq(t, 2, limit)
			assert.True(t, includePrerelease)
			return []sourcesf.LatestInfo{
				{
					Tag:         "tool-1.2.3",
					Version:     "1.2.3",
					Path:        "files/1.2.3",
					PublishedAt: time.Date(2026, 2, 3, 9, 21, 40, 0, time.UTC),
					Prerelease:  true,
					AssetsCount: 2,
				},
				{
					Version:     "1.2.2",
					Path:        "files/1.2.2",
					PublishedAt: time.Date(2026, 1, 3, 9, 21, 40, 0, time.UTC),
					AssetsCount: 1,
				},
			}, nil
		},
	}

	result, err := svc.Query(QueryOptions{Repo: "sourceforge:project/files", Action: "releases", Limit: 2, Prerelease: true})
	if err != nil {
		t.Fatalf("Query(): %v", err)
	}

	assert.Eq(t, "releases", result.Action)
	assert.Eq(t, "sourceforge:project/files", result.Repo)
	assert.Len(t, result.Releases, 2)
	assert.Eq(t, "tool-1.2.3", result.Releases[0].Tag)
	assert.Eq(t, "1.2.3", result.Releases[0].Name)
	assert.Eq(t, time.Date(2026, 2, 3, 9, 21, 40, 0, time.UTC), result.Releases[0].PublishedAt)
	assert.True(t, result.Releases[0].Prerelease)
	assert.Eq(t, 0, result.Releases[0].AssetsCount)
}

func TestQueryServiceSourceForgeRejectsUnsupportedActions(t *testing.T) {
	svc := QueryService{}

	if _, err := svc.Query(QueryOptions{Repo: "sourceforge:project", Action: "info"}); err == nil {
		t.Fatal("expected sourceforge info to fail")
	}
}

func TestQueryServiceDirectURL(t *testing.T) {
	t.Run("uses URL info for default action", func(t *testing.T) {
		const target = "https://example.com/tool.zip"
		calls := 0
		svc := QueryService{
			URLInfo: func(rawURL string) (QueryURLInfo, error) {
				calls++
				assert.Eq(t, target, rawURL)
				return QueryURLInfo{RequestedURL: rawURL, StatusCode: 200}, nil
			},
		}

		result, err := svc.Query(QueryOptions{Repo: target})

		assert.NoErr(t, err)
		assert.Eq(t, 1, calls)
		assert.Eq(t, "info", result.Action)
		assert.NotNil(t, result.URLInfo)
		if result.URLInfo != nil {
			assert.Eq(t, target, result.URLInfo.RequestedURL)
		}
	})

	t.Run("accepts explicit info action", func(t *testing.T) {
		svc := QueryService{
			URLInfo: func(rawURL string) (QueryURLInfo, error) {
				return QueryURLInfo{RequestedURL: rawURL}, nil
			},
		}

		result, err := svc.Query(QueryOptions{Repo: "https://example.com/tool.zip", Action: "info"})

		assert.NoErr(t, err)
		assert.Eq(t, "info", result.Action)
	})

	for _, action := range []string{"releases", "assets"} {
		t.Run("rejects "+action, func(t *testing.T) {
			svc := QueryService{}

			_, err := svc.Query(QueryOptions{Repo: "https://example.com/tool.zip", Action: action})

			assert.Err(t, err)
			assert.Contains(t, err.Error(), "direct URL query does not support")
		})
	}
}

func TestQueryServiceLatestUsesDefaultAction(t *testing.T) {
	client := &fakeQueryClient{
		releases: []QueryRelease{{
			Tag:         "v1.2.3",
			Name:        "v1.2.3",
			PublishedAt: time.Date(2026, 4, 22, 8, 0, 0, 0, time.UTC),
			AssetsCount: 3,
		}},
	}
	svc := QueryService{Client: client}

	result, err := svc.Query(QueryOptions{Repo: "owner/repo"})
	if err != nil {
		t.Fatalf("Query(): %v", err)
	}
	if result.Action != "latest" {
		t.Fatalf("expected default action latest, got %q", result.Action)
	}
	if client.latestCalls != 1 {
		t.Fatalf("expected LatestRelease to be called once, got %d", client.latestCalls)
	}
	if result.Latest == nil || result.Latest.Tag != "v1.2.3" {
		t.Fatalf("expected latest release in result, got %#v", result.Latest)
	}
}

func TestQueryServiceReleasesUsesLimit(t *testing.T) {
	client := &fakeQueryClient{
		releases: []QueryRelease{{Tag: "v1.2.3"}, {Tag: "v1.2.2"}},
	}
	svc := QueryService{Client: client}

	result, err := svc.Query(QueryOptions{Repo: "owner/repo", Action: "releases", Limit: 5})
	if err != nil {
		t.Fatalf("Query(): %v", err)
	}
	if client.listCalls != 1 || client.lastLimit != 5 {
		t.Fatalf("expected ListReleases(limit=5), got calls=%d limit=%d", client.listCalls, client.lastLimit)
	}
	if len(result.Releases) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(result.Releases))
	}
}

func TestQueryServiceAssetsUsesLatestTagWhenMissing(t *testing.T) {
	client := &fakeQueryClient{
		releases: []QueryRelease{{Tag: "v1.2.3"}},
		assets:   []QueryAsset{{Name: "tool-linux-amd64.tar.gz"}},
	}
	svc := QueryService{Client: client}

	result, err := svc.Query(QueryOptions{Repo: "owner/repo", Action: "assets"})
	if err != nil {
		t.Fatalf("Query(): %v", err)
	}
	if client.latestCalls != 1 {
		t.Fatalf("expected LatestRelease call, got %d", client.latestCalls)
	}
	if client.assetCalls != 1 || client.lastTag != "v1.2.3" {
		t.Fatalf("expected ReleaseAssets tag v1.2.3, got calls=%d tag=%q", client.assetCalls, client.lastTag)
	}
	if result.Tag != "v1.2.3" {
		t.Fatalf("expected resolved tag v1.2.3, got %q", result.Tag)
	}
}

func TestQueryServiceInfoReturnsRepoInfo(t *testing.T) {
	client := &fakeQueryClient{
		repoInfo: QueryRepoInfo{
			Repo:          "owner/repo",
			Description:   "test repo",
			DefaultBranch: "main",
		},
	}
	svc := QueryService{Client: client}

	result, err := svc.Query(QueryOptions{Repo: "owner/repo", Action: "info"})
	if err != nil {
		t.Fatalf("Query(): %v", err)
	}
	if client.repoCalls != 1 {
		t.Fatalf("expected RepoInfo call, got %d", client.repoCalls)
	}
	if result.Info == nil || result.Info.DefaultBranch != "main" {
		t.Fatalf("expected repo info result, got %#v", result.Info)
	}
}

func TestQueryServiceRejectsUnsupportedOptionUsage(t *testing.T) {
	svc := QueryService{Client: &fakeQueryClient{}}

	if _, err := svc.Query(QueryOptions{Repo: "owner/repo", Action: "info", Tag: "v1.2.3"}); err == nil {
		t.Fatal("expected info + tag to fail")
	}
	if _, err := svc.Query(QueryOptions{Repo: "owner/repo", Action: "latest", Limit: 3}); err == nil {
		t.Fatal("expected latest + limit to fail")
	}
}

func TestQueryResultJSONMarshals(t *testing.T) {
	result := QueryResult{
		Action: "latest",
		Repo:   "owner/repo",
		Latest: &QueryRelease{Tag: "v1.2.3"},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal(): %v", err)
	}
	if string(data) == "" {
		t.Fatal("expected non-empty json")
	}
}
