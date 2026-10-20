package cli

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	"github.com/inherelab/eget/internal/app"
	"github.com/inherelab/eget/internal/client"
)

func latestInfoResponse(status int, statusText, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     statusText,
		Body:       io.NopCloser(bytes.NewReader([]byte(body))),
		Header:     make(http.Header),
	}
}

// familyRefs formats refs for the matching-refs response; a leading "*" marks
// the refs/tags/ prefix.
func familyRefs(tags ...string) string {
	var parts []string
	for _, tag := range tags {
		parts = append(parts, `{"ref":"refs/tags/`+tag+`"}`)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func TestGithubTagLatestInfoFastPathHit(t *testing.T) {
	// family max == installed tag: the check settles with a single light
	// matching-refs request and never reads the release history
	var requested []string
	client := client.NewGitHubClientWithGetter(client.Options{}, func(rawURL string, opts client.Options) (*http.Response, error) {
		requested = append(requested, rawURL)
		if strings.Contains(rawURL, "/git/matching-refs/tags/cua-driver-rs-v") {
			return latestInfoResponse(http.StatusOK, "200 OK", familyRefs(
				"cua-driver-rs-v0.10.0", "cua-driver-rs-v0.36.0", "cua-driver-rs-v0.37.0")), nil
		}
		t.Fatalf("fast path hit must not read the release history: %s", rawURL)
		return nil, nil
	})

	got, err := githubTagLatestInfo(client, "trycua/cua", app.LatestCheckTarget{
		Tag:          "PRE:cua-driver-rs-v",
		InstalledTag: "cua-driver-rs-v0.37.0",
	})

	assert.NoErr(t, err)
	assert.Eq(t, "cua-driver-rs-v0.37.0", got.Tag)
	assert.True(t, got.PublishedAt.IsZero())
	for _, rawURL := range requested {
		if strings.Contains(rawURL, "/releases") {
			t.Fatalf("fast path hit must not read the release history: %s", rawURL)
		}
		if !strings.Contains(rawURL, "/git/matching-refs/") {
			t.Fatalf("expected only matching-refs requests, got %v", requested)
		}
	}
}

func TestGithubTagLatestInfoFastPathFallsBackToScan(t *testing.T) {
	// family max != installed tag: the release scan stays the authority for
	// what the latest release is and for its publish time
	var requested []string
	client := client.NewGitHubClientWithGetter(client.Options{}, func(rawURL string, opts client.Options) (*http.Response, error) {
		requested = append(requested, rawURL)
		if strings.Contains(rawURL, "/git/matching-refs/tags/cua-driver-rs-v") {
			return latestInfoResponse(http.StatusOK, "200 OK", familyRefs(
				"cua-driver-rs-v0.35.4", "cua-driver-rs-v0.37.0")), nil
		}
		if strings.Contains(rawURL, "/releases?") {
			body := `[
		 {"tag_name":"agent-v0.9.1","prerelease":false,"created_at":"2026-10-19T10:14:46Z","published_at":"2026-10-19T10:20:00Z"},
		 {"tag_name":"cua-driver-rs-v0.37.0","prerelease":false,"created_at":"2026-10-20T08:00:00Z","published_at":"2026-10-20T09:00:00Z"}]`
			return latestInfoResponse(http.StatusOK, "200 OK", body), nil
		}
		t.Fatalf("unexpected request: %s", rawURL)
		return nil, nil
	})

	got, err := githubTagLatestInfo(client, "trycua/cua", app.LatestCheckTarget{
		Tag:          "PRE:cua-driver-rs-v",
		InstalledTag: "cua-driver-rs-v0.35.4",
	})

	assert.NoErr(t, err)
	assert.Eq(t, "cua-driver-rs-v0.37.0", got.Tag)
	assert.Eq(t, time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC), got.PublishedAt)
	foundFamily, foundScan := false, false
	for _, rawURL := range requested {
		if strings.Contains(rawURL, "/git/matching-refs/") {
			foundFamily = true
		}
		if strings.Contains(rawURL, "/releases?") {
			foundScan = true
		}
	}
	if !foundFamily || !foundScan {
		t.Fatalf("expected family probe then release scan, got %v", requested)
	}
}

func TestGithubTagLatestInfoFailOpenEndpoints(t *testing.T) {
	cases := []struct {
		name     string
		refsResp func(rawURL string) *http.Response
	}{
		{"refs 500", func(rawURL string) *http.Response {
			return latestInfoResponse(http.StatusInternalServerError, "500 Internal Server Error", `{}`)
		}},
		{"refs empty", func(rawURL string) *http.Response {
			return latestInfoResponse(http.StatusOK, "200 OK", "[]")
		}},
		{"unorderable family", func(rawURL string) *http.Response {
			return latestInfoResponse(http.StatusOK, "200 OK", familyRefs(
				"cua-driver-rs-v0.37", "cua-driver-rs-v0.37.0"))
		}},
	}
	for _, c := range cases {
		client := client.NewGitHubClientWithGetter(client.Options{}, func(rawURL string, opts client.Options) (*http.Response, error) {
			if strings.Contains(rawURL, "/git/matching-refs/") {
				return c.refsResp(rawURL), nil
			}
			if strings.Contains(rawURL, "/releases?") {
				body := `[{"tag_name":"cua-driver-rs-v0.36.0","prerelease":false,"created_at":"2026-10-20T08:00:00Z","published_at":"2026-10-20T09:00:00Z"}]`
				return latestInfoResponse(http.StatusOK, "200 OK", body), nil
			}
			t.Fatalf("unexpected request: %s", rawURL)
			return nil, nil
		})

		got, err := githubTagLatestInfo(client, "trycua/cua", app.LatestCheckTarget{
			Tag:          "PRE:cua-driver-rs-v",
			InstalledTag: "cua-driver-rs-v0.35.4",
		})

		assert.NoErr(t, err, "case %s", c.name)
		assert.Eq(t, "cua-driver-rs-v0.36.0", got.Tag, "case %s", c.name)
	}
}

func TestGithubTagLatestInfoNoFamilyProbeWithoutInstalledTag(t *testing.T) {
	// self-update style callers carry no installed tag family knowledge: the
	// fast path must be skipped entirely
	client := client.NewGitHubClientWithGetter(client.Options{}, func(rawURL string, opts client.Options) (*http.Response, error) {
		if strings.Contains(rawURL, "/git/matching-refs/") {
			t.Fatalf("no family probe expected without installed tag: %s", rawURL)
		}
		if strings.Contains(rawURL, "/releases?") {
			body := `[{"tag_name":"cua-driver-rs-v0.37.0","prerelease":false,"created_at":"2026-10-20T08:00:00Z","published_at":"2026-10-20T09:00:00Z"}]`
			return latestInfoResponse(http.StatusOK, "200 OK", body), nil
		}
		t.Fatalf("unexpected request: %s", rawURL)
		return nil, nil
	})

	got, err := githubTagLatestInfo(client, "trycua/cua", app.LatestCheckTarget{
		Tag: "PRE:cua-driver-rs-v",
	})

	assert.NoErr(t, err)
	assert.Eq(t, "cua-driver-rs-v0.37.0", got.Tag)
}

func TestGithubTagLatestInfoResolvesTagPatternByReleaseScan(t *testing.T) {
	var requested []string
	client := client.NewGitHubClientWithGetter(client.Options{}, func(rawURL string, opts client.Options) (*http.Response, error) {
		requested = append(requested, rawURL)
		if strings.Contains(rawURL, "/releases?") {
			body := `[
			 {"tag_name":"agent-v0.9.1","prerelease":false,"created_at":"2026-10-19T10:14:46Z","published_at":"2026-10-19T10:20:00Z"},
			 {"tag_name":"cua-sdk-v0.5.0","prerelease":false,"created_at":"2026-10-19T05:28:20Z","published_at":"2026-10-19T05:30:00Z"},
			 {"tag_name":"cua-driver-rs-v0.36.0","prerelease":false,"created_at":"2026-10-20T08:00:00Z","published_at":"2026-10-20T09:00:00Z"},
			 {"tag_name":"cua-driver-rs-v0.35.4","prerelease":false,"created_at":"2026-10-14T12:32:00Z","published_at":"2026-10-14T12:32:14Z"}]`
			return latestInfoResponse(http.StatusOK, "200 OK", body), nil
		}
		t.Fatalf("unexpected request: %s", rawURL)
		return nil, nil
	})

	target := app.LatestCheckTarget{
		Repo:  "trycua/cua",
		Tag:   "PRE:cua-driver-rs-v",
		Asset: "cua-driver-rs-0.35.4-windows-x86_64-binary.zip",
	}
	got, err := githubTagLatestInfo(client, "trycua/cua", target)

	assert.NoErr(t, err)
	// the pattern is resolved from the newest-first release scan in a single
	// early-stopped page: no release-by-tag call (which would 404 for a
	// pattern) and no extra follow-up lookup of the matched release
	if len(requested) != 1 {
		t.Fatalf("expected 1 API call for the early-stopped scan, got %d: %v", len(requested), requested)
	}
	if strings.Contains(requested[0], "/releases/tags/") {
		t.Fatalf("pattern must not be sent to release-by-tag: %s", requested[0])
	}
	assert.Eq(t, "cua-driver-rs-v0.36.0", got.Tag)
	assert.Eq(t, time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC), got.PublishedAt)
}

func TestGithubTagLatestInfoReportsNoTagMatch(t *testing.T) {
	client := client.NewGitHubClientWithGetter(client.Options{}, func(rawURL string, opts client.Options) (*http.Response, error) {
		if strings.Contains(rawURL, "/releases?") {
			body := `[{"tag_name":"agent-v0.9.1","prerelease":false,"created_at":"2026-10-19T10:14:46Z"}]`
			return latestInfoResponse(http.StatusOK, "200 OK", body), nil
		}
		t.Fatalf("unexpected request: %s", rawURL)
		return nil, nil
	})

	target := app.LatestCheckTarget{Repo: "trycua/cua", Tag: "PRE:cua-driver-rs-v"}
	_, err := githubTagLatestInfo(client, "trycua/cua", target)

	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "no matching tag"), "err: %v", err)
}

func TestGithubTagLatestInfoKeepsPlainTagBehavior(t *testing.T) {
	var callCount int
	client := client.NewGitHubClientWithGetter(client.Options{}, func(rawURL string, opts client.Options) (*http.Response, error) {
		callCount++
		if !strings.Contains(rawURL, "/releases/tags/v9.9.9") {
			t.Fatalf("unexpected request: %s", rawURL)
		}
		if callCount == 1 {
			body := `{"tag_name":"v9.9.9","created_at":"2026-04-20T14:10:17Z","published_at":"2026-04-21T14:10:17Z"}`
			return latestInfoResponse(http.StatusOK, "200 OK", body), nil
		}
		body := `{"assets":[{"id":7,"name":"tool-windows-x86_64.zip","size":42,"updated_at":"2026-04-22T10:00:00Z","digest":"sha256:abc","browser_download_url":"https://example.com/tool"}]}`
		return latestInfoResponse(http.StatusOK, "200 OK", body), nil
	})

	target := app.LatestCheckTarget{Repo: "owner/repo", Tag: "v9.9.9", Asset: "tool-windows-x86_64.zip"}
	got, err := githubTagLatestInfo(client, "owner/repo", target)

	assert.NoErr(t, err)
	assert.Eq(t, "v9.9.9", got.Tag)
	assert.Eq(t, "tool-windows-x86_64.zip", got.AssetName)
	assert.Eq(t, "sha256:abc", got.AssetDigest)
	if callCount != 2 {
		t.Fatalf("expected 2 API calls (release info + assets), got %d", callCount)
	}
}
