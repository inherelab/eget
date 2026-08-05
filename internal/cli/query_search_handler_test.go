package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gookit/cliui"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	clirender "github.com/inherelab/eget/internal/cli/render"
)

func TestHandleQueryPrintsLatestRelease(t *testing.T) {
	svc := &cliService{
		queryService: app.QueryService{
			Client: &fakeQueryClientForCLI{
				releases: []app.QueryRelease{{
					Tag:         "v1.2.3",
					Name:        "v1.2.3",
					PublishedAt: time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC),
					AssetsCount: 2,
				}},
			},
		},
	}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	os.Stdout = w
	cliui.SetOutput(w)
	defer func() {
		os.Stdout = origStdout
		cliui.ResetOutput()
	}()

	err = svc.handleQuery(&QueryOptions{Target: "owner/repo"})
	if err != nil {
		t.Fatalf("handle query: %v", err)
	}

	_ = w.Close()
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "action: latest") || !strings.Contains(got, "repo: owner/repo") {
		t.Fatalf("expected latest query output, got %q", got)
	}
	if !strings.Contains(got, "2026-04-22 09:00:00") {
		t.Fatalf("expected compact published time in latest output, got %q", got)
	}
	if strings.Contains(got, "2026-04-22T09:00:00") {
		t.Fatalf("expected latest output with a space between date and time, got %q", got)
	}
}

func TestPrintQueryResultReleasesUsesCompactTime(t *testing.T) {
	result := app.QueryResult{
		Action: "releases",
		Repo:   "owner/repo",
		Releases: []app.QueryRelease{{
			Tag:         "v1.2.3",
			Name:        "v1.2.3",
			PublishedAt: time.Date(2026, 4, 22, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60)),
			AssetsCount: 2,
		}},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	clirender.PrintQueryResult(result)
	got := out.String()
	if !strings.Contains(got, "2026-04-22 09:00:00") {
		t.Fatalf("expected compact published time in releases output, got %q", got)
	}
	if strings.Contains(got, "2026-04-22T09:00:00") {
		t.Fatalf("expected releases output with a space between date and time, got %q", got)
	}
}

func TestPrintQueryResultSourceForgeReleasesShowsUnknownAssetsCount(t *testing.T) {
	result := app.QueryResult{
		Action: "releases",
		Repo:   "sourceforge:project/path",
		Releases: []app.QueryRelease{{
			Tag:  "tool-1.2.3",
			Name: "1.2.3",
		}},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	clirender.PrintQueryResult(result)
	got := out.String()
	if !strings.Contains(got, "Assets Count") || !strings.Contains(got, " - ") {
		t.Fatalf("expected unknown assets count marker in sourceforge releases output, got %q", got)
	}
	if strings.Contains(got, " 0 ") {
		t.Fatalf("expected sourceforge releases output to hide zero assets count, got %q", got)
	}
}

func TestPrintQueryResultInfoUsesCompactTime(t *testing.T) {
	result := app.QueryResult{
		Action: "info",
		Repo:   "owner/repo",
		Info: &app.QueryRepoInfo{
			Repo:      "owner/repo",
			UpdatedAt: time.Date(2026, 4, 22, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60)),
		},
	}

	var out bytes.Buffer
	cliui.SetOutput(&out)
	defer cliui.ResetOutput()

	clirender.PrintQueryResult(result)
	got := out.String()
	if !strings.Contains(got, "2026-04-22 09:00:00") {
		t.Fatalf("expected compact updated time in info output, got %q", got)
	}
	if strings.Contains(got, "2026-04-22T09:00:00") {
		t.Fatalf("expected info output with a space between date and time, got %q", got)
	}
}

func TestQueryResultURLInfo(t *testing.T) {
	size := int64(2048)
	result := app.QueryResult{
		Action: "info",
		URLInfo: &app.QueryURLInfo{
			RequestedURL: "https://example.com/start",
			FinalURL:     "https://cdn.example.com/tool.zip",
			Status:       "200 OK",
			StatusCode:   200,
			FileName:     "tool.zip",
			Size:         &size,
			ContentType:  "application/zip",
			LastModified: "Tue, 05 Aug 2026 10:00:00 GMT",
			ETag:         `"abc123"`,
			AcceptRanges: "bytes",
		},
	}

	t.Run("prints text fields and readable size", func(t *testing.T) {
		var out bytes.Buffer
		cliui.SetOutput(&out)
		defer cliui.ResetOutput()

		clirender.PrintQueryResult(result)

		got := out.String()
		for _, want := range []string{
			result.URLInfo.RequestedURL,
			result.URLInfo.FinalURL,
			result.URLInfo.Status,
			result.URLInfo.FileName,
			"2.00K",
			result.URLInfo.ContentType,
			result.URLInfo.LastModified,
			result.URLInfo.ETag,
			result.URLInfo.AcceptRanges,
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("expected URL info output to contain %q, got %q", want, got)
			}
		}
	})

	t.Run("renders JSON with raw byte size", func(t *testing.T) {
		got, err := clirender.QueryResultJSON(result)
		if err != nil {
			t.Fatalf("query result JSON: %v", err)
		}
		for _, want := range []string{`"url_info"`, `"size": 2048`, result.URLInfo.FinalURL} {
			if !strings.Contains(got, want) {
				t.Fatalf("expected URL info JSON to contain %q, got %q", want, got)
			}
		}
	})

	t.Run("omits unknown size from JSON", func(t *testing.T) {
		result.URLInfo.Size = nil
		got, err := clirender.QueryResultJSON(result)
		if err != nil {
			t.Fatalf("query result JSON: %v", err)
		}
		if strings.Contains(got, `"size"`) {
			t.Fatalf("expected unknown size to be omitted, got %q", got)
		}
	})
}

func TestHandleQueryJSONOutput(t *testing.T) {
	svc := &cliService{
		queryService: app.QueryService{
			Client: &fakeQueryClientForCLI{
				releases: []app.QueryRelease{{
					Tag:         "v1.2.3",
					PublishedAt: time.Date(2026, 4, 22, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60)),
				}},
			},
		},
	}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	err = svc.handleQuery(&QueryOptions{Target: "owner/repo", JSON: true})
	if err != nil {
		t.Fatalf("handle query: %v", err)
	}

	_ = w.Close()
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"action": "latest"`) {
		t.Fatalf("expected json query output, got %q", got)
	}
	if !strings.Contains(got, `"published_at": "2026-04-22T09:00:00"`) {
		t.Fatalf("expected compact json query time, got %q", got)
	}
	if strings.Contains(got, "2026-04-22T09:00:00+08:00") {
		t.Fatalf("expected json query time without timezone offset, got %q", got)
	}
}

func TestPrintQueryResultAssets(t *testing.T) {
	result := app.QueryResult{
		Action: "assets",
		Repo:   "owner/repo",
		Tag:    "v1.2.3",
		Assets: []app.QueryAsset{{
			Name: "tool-linux-amd64.tar.gz",
			URL:  "https://example.com/tool",
		}},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	clirender.PrintQueryResult(result)
	if !strings.Contains(out.String(), "tool-linux-amd64.tar.gz") {
		t.Fatalf("expected asset table output, got %q", out.String())
	}
}

func TestHandleSearchPrintsList(t *testing.T) {
	svc := &cliService{
		searchService: app.SearchService{
			Client: &fakeSearchClientForCLI{
				result: app.SearchResult{
					TotalCount: 1,
					Items: []app.SearchRepo{{
						FullName:        "BurntSushi/ripgrep",
						Description:     "ripgrep recursively searches directories",
						StargazersCount: 123,
						Language:        "Rust",
						UpdatedAt:       time.Date(2026, 4, 24, 8, 30, 0, 0, time.UTC),
					}},
				},
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handle("search", &SearchOptions{Keyword: "ripgrep", Extras: []string{"language:rust"}})
	if err != nil {
		t.Fatalf("handle search: %v", err)
	}

	got := out.String()
	if strings.Contains(strings.ToLower(got), "repo |") || strings.Contains(strings.ToLower(got), "language |") {
		t.Fatalf("expected search to not render a table, got %q", got)
	}
	if !strings.Contains(got, "BurntSushi/ripgrep") || !strings.Contains(got, "⭐123 language: Rust update: 2026-04-24 08:30:00") {
		t.Fatalf("expected formatted search headline, got %q", got)
	}
	if strings.Contains(got, "2026-04-24T08:30:00") {
		t.Fatalf("expected search time with a space between date and time, got %q", got)
	}
	if !strings.Contains(got, "ripgrep recursively searches directories") {
		t.Fatalf("expected description line, got %q", got)
	}
}

func TestHandleSearchJSONOutput(t *testing.T) {
	svc := &cliService{
		searchService: app.SearchService{
			Client: &fakeSearchClientForCLI{
				result: app.SearchResult{
					TotalCount: 1,
					Items: []app.SearchRepo{{
						FullName:        "BurntSushi/ripgrep",
						StargazersCount: 321,
						UpdatedAt:       time.Date(2026, 4, 24, 8, 30, 0, 0, time.FixedZone("CST", 8*60*60)),
					}},
				},
			},
		},
	}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	err = svc.handle("search", &SearchOptions{Keyword: "ripgrep", JSON: true})
	if err != nil {
		t.Fatalf("handle search json: %v", err)
	}

	_ = w.Close()
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"total_count": 1`) || !strings.Contains(got, `"full_name": "BurntSushi/ripgrep"`) {
		t.Fatalf("expected search json output, got %q", got)
	}
	if !strings.Contains(got, `"updated_at": "2026-04-24T08:30:00"`) {
		t.Fatalf("expected compact search json time, got %q", got)
	}
	if strings.Contains(got, "2026-04-24T08:30:00+08:00") {
		t.Fatalf("expected search json time without timezone offset, got %q", got)
	}
}

func TestHandleSearchPassesOptionsToSearchService(t *testing.T) {
	fakeClient := &fakeSearchClientForCLI{
		result: app.SearchResult{},
	}
	svc := &cliService{
		searchService: app.SearchService{
			Client: fakeClient,
		},
	}

	err := svc.handle("search", &SearchOptions{
		Keyword: "ripgrep",
		Limit:   20,
		Sort:    "stars",
		Order:   "desc",
		Extras:  []string{"language:rust", "topic:cli"},
	})
	if err != nil {
		t.Fatalf("handle search: %v", err)
	}

	if fakeClient.query != "ripgrep language:rust topic:cli" {
		t.Fatalf("expected merged query to propagate, got %q", fakeClient.query)
	}
	if fakeClient.limit != 20 {
		t.Fatalf("expected limit to propagate, got %d", fakeClient.limit)
	}
	if fakeClient.sort != "stars" {
		t.Fatalf("expected sort to propagate, got %q", fakeClient.sort)
	}
	if fakeClient.order != "desc" {
		t.Fatalf("expected order to propagate, got %q", fakeClient.order)
	}
}
