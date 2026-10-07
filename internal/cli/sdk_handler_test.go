package cli

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/sdk"
)

func TestHandleSDKInstallPrintsResults(t *testing.T) {
	fake := &fakeSDKService{
		installResults: []sdk.InstallResult{{
			Name: "go", Version: "1.21.1", Path: "/sdks/go1.21.1", Cached: true, Resumed: true,
		}},
	}
	svc := &cliService{sdkService: fake}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handle("sdk.install", &SDKInstallOptions{Targets: []string{"go@1.21.1"}, Force: true})
	assert.NoErr(t, err)
	assert.Eq(t, []string{"go@1.21.1"}, fake.installTargets)
	assert.True(t, fake.installOpts.Force)
	assert.NotNil(t, fake.installOpts.Progress)
	got := out.String()
	assert.Contains(t, got, "Install SDK go@1.21.1 -> 1.21.1 from example.com")
	assert.Contains(t, got, "Use cached SDK archive: /cache/go.zip")
	assert.Contains(t, got, "Extract SDK archive: /cache/go.zip -> /sdks/go1.21.1")
	assert.Contains(t, got, "go@1.21.1")
	assert.Contains(t, got, "/sdks/go1.21.1")
	assert.Contains(t, got, "cached")
	assert.Contains(t, got, "resumed")
}

func TestHandleSDKDownloadPrintsResults(t *testing.T) {
	fake := &fakeSDKService{
		downloadResults: []sdk.SDKDownloadResult{{
			Name: "go", Version: "1.21.1", Path: "/cache/go.zip", OS: "windows", Arch: "amd64", Cached: true, Resumed: true,
		}},
	}
	svc := &cliService{sdkService: fake}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handle("sdk.download", &SDKDownloadOptions{Targets: []string{"go@1.21.1"}, OS: "windows", Arch: "amd64", Output: "downloads"})
	assert.NoErr(t, err)
	assert.Eq(t, []string{"go@1.21.1"}, fake.downloadTargets)
	assert.Eq(t, "windows", fake.downloadOpts.Platform.OS)
	assert.Eq(t, "amd64", fake.downloadOpts.Platform.Arch)
	assert.Eq(t, "downloads", fake.downloadOpts.OutputDir)
	assert.NotNil(t, fake.downloadOpts.Progress)
	got := out.String()
	assert.Contains(t, got, "Download SDK go@1.21.1 -> 1.21.1 (windows/amd64) from example.com")
	assert.Contains(t, got, "go@1.21.1")
	assert.Contains(t, got, "windows/amd64")
	assert.Contains(t, got, "/cache/go.zip")
	assert.Contains(t, got, "cached")
	assert.Contains(t, got, "resumed")
}

func TestHandleSDKDownloadRejectsMissingTarget(t *testing.T) {
	svc := &cliService{sdkService: &fakeSDKService{}}

	err := svc.handle("sdk.download", &SDKDownloadOptions{})

	assert.Err(t, err)
	assert.Contains(t, err.Error(), "target is required")
}

func TestHandleSDKListJSONOutput(t *testing.T) {
	fake := &fakeSDKService{
		listEntries: []sdk.InstalledEntry{{
			Name: "go", Version: "1.21.1", Path: "/sdks/go1.21.1", InstalledAt: time.Date(2026, 5, 17, 9, 0, 0, 0, time.UTC),
		}},
	}
	svc := &cliService{sdkService: fake}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	assert.NoErr(t, err)
	defer r.Close()
	defer w.Close()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	err = svc.handle("sdk.list", &SDKListOptions{Name: "go", JSON: true})
	assert.NoErr(t, err)
	assert.Eq(t, "go", fake.listName)

	_ = w.Close()
	var out bytes.Buffer
	_, err = io.Copy(&out, r)
	assert.NoErr(t, err)
	got := out.String()
	assert.Contains(t, got, `"name": "go"`)
	assert.Contains(t, got, `"installed_at": "2026-05-17T09:00:00"`)
}

func TestHandleSDKRemovePrintsResult(t *testing.T) {
	fake := &fakeSDKService{
		removeResult: sdk.RemoveResult{Name: "go", Version: "1.21.1", Path: "/sdks/go1.21.1", Missing: true},
	}
	svc := &cliService{sdkService: fake}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handle("sdk.remove", &SDKRemoveOptions{Target: "go@1.21.1"})
	assert.NoErr(t, err)
	assert.Eq(t, "go@1.21.1", fake.removeTarget)
	assert.Contains(t, out.String(), "already missing")
}

func TestHandleSDKSearchPrintsResults(t *testing.T) {
	fake := &fakeSDKService{
		searchResults: []sdk.SearchResult{{
			SDK: "go", Version: "1.22.0", Stable: true, OS: "linux", Arch: "amd64", Ext: "tar.gz", Filename: "go1.22.0.linux-amd64.tar.gz",
		}},
	}
	svc := &cliService{sdkService: fake}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handle("sdk.search", &SDKSearchOptions{Name: "go", Keywords: []string{"1.22", "amd64", "^windows"}, Number: 7, Sort: "desc", Prerelease: true})
	assert.NoErr(t, err)
	assert.Eq(t, "go", fake.searchName)
	assert.Eq(t, []string{"1.22", "amd64", "^windows"}, fake.searchKeywords)
	assert.Eq(t, 7, fake.searchNumber)
	assert.Eq(t, "desc", fake.searchSort)
	assert.True(t, fake.searchPrerelease)
	got := out.String()
	assert.Contains(t, got, "1.22.0")
	assert.Contains(t, got, "linux")
	assert.Contains(t, got, "go1.22.0.linux-amd64.tar.gz")
}

func TestHandleSDKIndexActions(t *testing.T) {
	now := time.Date(2026, 5, 17, 9, 0, 0, 0, time.UTC)
	fake := &fakeSDKService{
		index: sdk.Index{
			Schema:    1,
			SDK:       "go",
			SourceURL: "https://example.com/go",
			FetchedAt: now,
			Items: []sdk.IndexItem{
				{Version: "1.21.1", Stable: true, Files: []sdk.IndexFile{
					{OS: "linux", Arch: "amd64", Ext: "tar.gz", Filename: "go1.21.1.linux-amd64.tar.gz"},
					{OS: "windows", Arch: "amd64", Ext: "zip", Filename: "go1.21.1.windows-amd64.zip"},
				}},
				{Version: "1.22.0-rc.1", Stable: false, Files: []sdk.IndexFile{
					{OS: "linux", Arch: "amd64", Ext: "tar.gz", Filename: "go1.22.0-rc.1.linux-amd64.tar.gz"},
				}},
			},
		},
		indexes: []sdk.Index{
			{Schema: 1, SDK: "go", SourceURL: "https://example.com/go", FetchedAt: now},
		},
		cachedIndexes: []sdk.CachedIndexInfo{
			{SDK: "go", Versions: 3, SourceURL: "https://example.com/sdks/go/releases/download/archive/index/list/with/a/very/long/path", FetchedAt: now, Cached: true},
		},
	}
	svc := &cliService{sdkService: fake}

	assert.NoErr(t, svc.handle("sdk.index.refresh", &SDKIndexOptions{Action: "refresh", Name: "go"}))
	assert.Eq(t, "go", fake.indexName)
	assert.NoErr(t, svc.handle("sdk.index.refresh", &SDKIndexOptions{Action: "refresh", All: true}))
	assert.True(t, fake.indexAll)
	assert.NoErr(t, svc.handle("sdk.index.clear", &SDKIndexOptions{Action: "clear", Name: "go"}))
	assert.Eq(t, "go", fake.clearName)
	assert.NoErr(t, svc.handle("sdk.index.clear", &SDKIndexOptions{Action: "clear", All: true}))
	assert.True(t, fake.clearAll)

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)
	assert.NoErr(t, svc.handle("sdk.index.list", &SDKIndexOptions{Action: "list"}))
	listOut := ccolor.ClearCode(out.String())
	assert.Contains(t, listOut, "go")
	assert.Contains(t, listOut, "...")
	assert.NotContains(t, listOut, "/with/a/very/long/path")

	out.Reset()
	assert.NoErr(t, svc.handle("sdk.index.show", &SDKIndexOptions{Action: "show", Name: "go"}))
	showOut := out.String()
	assert.Contains(t, showOut, "SDK Index")
	assert.Contains(t, showOut, "go")
	assert.Contains(t, showOut, "Versions")
	assert.Contains(t, showOut, "Latest Stable")
	assert.NotContains(t, showOut, " true ")
	assert.NotContains(t, showOut, " false ")
	assert.NotContains(t, showOut, "Version | Stable | Files")
	assert.NotContains(t, showOut, `"items"`)
}

func TestHandleSDKIndexListShowsConfiguredMissingCache(t *testing.T) {
	fake := &fakeSDKService{
		cachedIndexes: []sdk.CachedIndexInfo{
			{SDK: "go", SourceURL: "https://go.dev/dl/"},
		},
	}
	svc := &cliService{sdkService: fake}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	assert.NoErr(t, svc.handle("sdk.index.list", &SDKIndexOptions{Action: "list"}))
	got := ccolor.ClearCode(out.String())
	assert.Contains(t, got, "go")
	assert.Contains(t, got, "https://go.dev/dl/")
	assert.Contains(t, got, " - ")
}

func TestHandleSDKConfigAddPrintsResult(t *testing.T) {
	cfg := cfgpkg.NewFile()
	svc := &cliService{
		cfgService: app.ConfigService{
			Load: func() (*cfgpkg.File, error) { return cfg, nil },
			Save: func(path string, file *cfgpkg.File) error { return nil },
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handle("sdk.config.add", &SDKConfigOptions{Action: "add", Name: "jdk", Mirror: "zulu"})
	if err != nil {
		t.Fatalf("handle sdk config add: %v", err)
	}

	got := ccolor.ClearCode(out.String())
	assert.Contains(t, got, "Added SDK config: jdk (zulu)")
	assert.Eq(t, "zulu-json", *cfg.SDK["jdk"].IndexParser)
}

func TestHandleSDKConfigAddAllPrintsSkippedAndAdded(t *testing.T) {
	cfg := cfgpkg.NewFile()
	cfg.SDK["go"] = cfgpkg.SDKSection{Target: cliStringPtr("custom-go")}
	svc := &cliService{
		cfgService: app.ConfigService{
			Load: func() (*cfgpkg.File, error) { return cfg, nil },
			Save: func(path string, file *cfgpkg.File) error { return nil },
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handle("sdk.config.add", &SDKConfigOptions{Action: "add", All: true, Mirror: "mirror"})
	if err != nil {
		t.Fatalf("handle sdk config add all: %v", err)
	}

	got := ccolor.ClearCode(out.String())
	assert.Contains(t, got, "Skipped SDK config: go already exists")
	assert.Contains(t, got, "Added SDK config: node (mirror)")
	assert.Contains(t, got, "Added SDK config: jdk (mirror)")
}

func TestHandleSDKPathPrintsPath(t *testing.T) {
	fake := &fakeSDKService{
		pathEntry: sdk.InstalledEntry{Name: "jdk", Version: "17.0.11", Path: "D:/tools/jdk/zulu-17.0.11"},
	}
	svc := &cliService{sdkService: fake}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handle("sdk.path", &SDKPathOptions{Target: "java:17"})
	assert.NoErr(t, err)
	assert.Eq(t, "java:17", fake.pathTarget)
	assert.Eq(t, "D:/tools/jdk/zulu-17.0.11\n", out.String())
}
