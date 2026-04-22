package cli

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	"github.com/inherelab/eget/internal/install"
)

func TestGitHubQueryClientLatestRelease(t *testing.T) {
	client := newGitHubQueryClient(install.Options{})
	client.get = func(url string, opts install.Options) (*http.Response, error) {
		body := `{"tag_name":"v1.2.3","name":"v1.2.3","prerelease":false,"published_at":"2026-04-22T10:00:00Z","assets":[{},{}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}, nil
	}

	got, err := client.LatestRelease("owner/repo", false)
	if err != nil {
		t.Fatalf("LatestRelease(): %v", err)
	}
	if got.Tag != "v1.2.3" || got.AssetsCount != 2 {
		t.Fatalf("unexpected latest release: %#v", got)
	}
}

func TestGitHubQueryClientReleaseAssets(t *testing.T) {
	client := newGitHubQueryClient(install.Options{})
	client.get = func(url string, opts install.Options) (*http.Response, error) {
		body := `{"assets":[{"name":"tool-linux-amd64.tar.gz","size":12,"download_count":3,"updated_at":"2026-04-22T10:00:00Z","browser_download_url":"https://example.com/tool"}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}, nil
	}

	got, err := client.ReleaseAssets("owner/repo", "v1.2.3")
	if err != nil {
		t.Fatalf("ReleaseAssets(): %v", err)
	}
	if len(got) != 1 || got[0].Name != "tool-linux-amd64.tar.gz" {
		t.Fatalf("unexpected assets: %#v", got)
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

	printQueryResult(result)
	if !strings.Contains(out.String(), "tool-linux-amd64.tar.gz") {
		t.Fatalf("expected asset table output, got %q", out.String())
	}
}
