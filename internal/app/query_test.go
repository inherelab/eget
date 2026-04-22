package app

import (
	"encoding/json"
	"testing"
	"time"
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
