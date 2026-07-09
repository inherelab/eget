package app

import (
	"errors"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
)

func TestInstallTargetRunsInstallFlowAndRecordsInstalledState(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/junegunn/fzf/releases/download/v1.0.0/fzf.tar.gz",
			Tool:           "fzf",
			AssetID:        100,
			AssetSize:      12,
			AssetUpdatedAt: now.Add(-30 * time.Minute),
			AssetDigest:    "sha256:abc",
			ExtractedFiles: []string{"./fzf"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{
		Runner: runner,
		Store:  store,
		Now: func() time.Time {
			return now
		},
		ReleaseInfo: func(repo, url string) (string, time.Time, error) {
			if repo != "junegunn/fzf" {
				t.Fatalf("expected repo junegunn/fzf, got %q", repo)
			}
			return "v1.0.0", now.Add(-time.Hour), nil
		},
		RepoMetadata: func(repo string) (RepoMetadata, error) {
			assert.Eq(t, "junegunn/fzf", repo)
			return RepoMetadata{
				Desc:     "Command-line fuzzy finder",
				Homepage: "https://junegunn.github.io/fzf",
				RepoURL:  "https://github.com/junegunn/fzf",
			}, nil
		},
	}

	opts := install.Options{
		System:      "linux/amd64",
		Output:      "~/.local/bin",
		ExtractFile: "fzf",
		Asset:       []string{"linux"},
		Tag:         "v1.0.0",
		Verify:      "abc123",
		Source:      true,
		DisableSSL:  true,
		All:         true,
		Prerelease:  true,
	}

	result, err := svc.InstallTarget("junegunn/fzf", opts)
	if err != nil {
		t.Fatalf("install target: %v", err)
	}

	if runner.calls != 1 {
		t.Fatalf("expected runner to be called once, got %d", runner.calls)
	}
	if runner.target != "junegunn/fzf" {
		t.Fatalf("expected target junegunn/fzf, got %q", runner.target)
	}
	if store.calls != 1 {
		t.Fatalf("expected store to be called once, got %d", store.calls)
	}
	if store.target != "fzf" {
		t.Fatalf("expected store target fzf, got %q", store.target)
	}
	if store.entry.Tool != "fzf" {
		t.Fatalf("expected tool fzf, got %q", store.entry.Tool)
	}
	if store.entry.Tag != "v1.0.0" {
		t.Fatalf("expected tag v1.0.0, got %q", store.entry.Tag)
	}
	assert.Eq(t, int64(100), store.entry.AssetID)
	assert.Eq(t, int64(12), store.entry.AssetSize)
	assert.Eq(t, now.Add(-30*time.Minute), store.entry.AssetUpdatedAt)
	assert.Eq(t, "sha256:abc", store.entry.AssetDigest)
	if store.entry.InstalledAt != now {
		t.Fatalf("expected installed at %v, got %v", now, store.entry.InstalledAt)
	}
	if got := store.entry.Options["system"]; got != "linux/amd64" {
		t.Fatalf("expected system option to be recorded, got %#v", got)
	}
	if got := store.entry.Options["download_source"]; got != true {
		t.Fatalf("expected source option to be recorded, got %#v", got)
	}
	if got := store.entry.Options["prerelease"]; got != true {
		t.Fatalf("expected prerelease option to be recorded, got %#v", got)
	}
	assert.Eq(t, "Command-line fuzzy finder", store.entry.Desc)
	assert.Eq(t, "https://junegunn.github.io/fzf", store.entry.Homepage)
	assert.Eq(t, "https://github.com/junegunn/fzf", store.entry.RepoURL)
	if len(result.ExtractedFiles) != 1 || result.ExtractedFiles[0] != "./fzf" {
		t.Fatalf("expected extracted files to round-trip, got %#v", result.ExtractedFiles)
	}
}

func TestInstallTargetRecordsTagFromReleaseURLBeforeLatestFallback(t *testing.T) {
	releaseInfoCalls := 0
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/junegunn/fzf/releases/download/v1.0.0/fzf.tar.gz",
			Tool:           "fzf",
			ExtractedFiles: []string{"./fzf"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{
		Runner: runner,
		Store:  store,
		ReleaseInfo: func(repo, url string) (string, time.Time, error) {
			releaseInfoCalls++
			return "v9.9.9", time.Unix(1710000000, 0).UTC(), nil
		},
	}

	_, err := svc.InstallTarget("junegunn/fzf", install.Options{})
	if err != nil {
		t.Fatalf("install target: %v", err)
	}

	if store.entry.Tag != "v1.0.0" {
		t.Fatalf("expected tag from release URL, got %q", store.entry.Tag)
	}
	assert.Eq(t, 0, releaseInfoCalls)
}

func TestInstallTargetRecordsTagPolicy(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/AandStep/ResultV/releases/download/v3.2.5/ResultV.AppImage",
			Tool:           "ResultV",
			ExtractedFiles: []string{"./ResultV.AppImage"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{Runner: runner, Store: store}

	_, err := svc.InstallTarget("AandStep/ResultV", install.Options{Tag: "v3.2.5"})

	assert.NoErr(t, err)
	assert.Eq(t, "latest", store.entry.TagPolicy)
	assert.Eq(t, "latest", store.entry.Options["tag_policy"])
}

func TestInstallTargetRecordsTrackTagPolicy(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/AandStep/ResultV/releases/download/v3.2.5/ResultV.AppImage",
			Tool:           "ResultV",
			ExtractedFiles: []string{"./ResultV.AppImage"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{Runner: runner, Store: store}

	_, err := svc.InstallTarget("AandStep/ResultV", install.Options{Tag: "v3.2.5", TagPolicy: "tag"})

	assert.NoErr(t, err)
	assert.Eq(t, "tag", store.entry.TagPolicy)
	assert.Eq(t, "tag", store.entry.Options["tag_policy"])
}

func TestInstallTargetRecordsSourceForgeVersionFromURL(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://downloads.sourceforge.net/project/winmerge/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe",
			Tool:           "winmerge",
			ExtractedFiles: []string{"./WinMerge.exe"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{
		Runner: runner,
		Store:  store,
	}

	_, err := svc.InstallTarget("sourceforge:winmerge", install.Options{SourcePath: "stable"})
	if err != nil {
		t.Fatalf("install sourceforge target: %v", err)
	}

	if store.entry.Repo != "sourceforge:winmerge" {
		t.Fatalf("expected normalized sourceforge repo, got %q", store.entry.Repo)
	}
	if store.entry.Tag != "2.16.44" || store.entry.Version != "2.16.44" {
		t.Fatalf("expected sourceforge version 2.16.44, got tag=%q version=%q", store.entry.Tag, store.entry.Version)
	}
}

func TestInstallTargetRecordsForgeVersionFromReleaseInfo(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://gitlab.com/fdroid/fdroidserver/-/releases/v2.3.3/downloads/fdroidserver-linux-amd64.tar.gz",
			Tool:           "fdroidserver",
			ExtractedFiles: []string{"./fdroidserver"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{
		Runner: runner,
		Store:  store,
		ReleaseInfo: func(repo, url string) (string, time.Time, error) {
			if repo != "gitlab:gitlab.com/fdroid/fdroidserver" {
				t.Fatalf("unexpected repo %q", repo)
			}
			return "v9.9.9", now.Add(-time.Hour), nil
		},
	}

	_, err := svc.InstallTarget("gitlab:fdroid/fdroidserver", install.Options{})
	if err != nil {
		t.Fatalf("install forge target: %v", err)
	}

	assert.Eq(t, "gitlab:gitlab.com/fdroid/fdroidserver", store.entry.Repo)
	assert.Eq(t, "v2.3.3", store.entry.Tag)
	assert.Eq(t, "v2.3.3", store.entry.Version)
}

func TestInstallTargetRecordsForgeTagFromOptionsBeforeLatestFallback(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://gitlab.com/fdroid/fdroidserver/-/package_files/123/download",
			Tool:           "fdroidserver",
			ExtractedFiles: []string{"./fdroidserver"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{
		Runner: runner,
		Store:  store,
		ReleaseInfo: func(repo, url string) (string, time.Time, error) {
			return "v9.9.9", now.Add(-time.Hour), nil
		},
	}

	_, err := svc.InstallTarget("gitlab:fdroid/fdroidserver", install.Options{Tag: "v2.3.3"})
	if err != nil {
		t.Fatalf("install pinned forge target: %v", err)
	}

	assert.Eq(t, "gitlab:gitlab.com/fdroid/fdroidserver", store.entry.Repo)
	assert.Eq(t, "v2.3.3", store.entry.Tag)
	assert.Eq(t, "v2.3.3", store.entry.Version)
}

func TestInstallTargetSkipsGitHubMetadataForTemplatePackage(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "http://mirror.kdev.com/tools/miglite/miglite-windows-amd64.exe",
			Tool:           "miglite.exe",
			ExtractedFiles: []string{"miglite.exe"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{
		Runner: runner,
		Store:  store,
		ReleaseInfo: func(repo, url string) (string, time.Time, error) {
			t.Fatalf("template package should not request release info, got repo=%q url=%q", repo, url)
			return "", time.Time{}, nil
		},
		RepoMetadata: func(repo string) (RepoMetadata, error) {
			t.Fatalf("template package should not request repo metadata, got repo=%q", repo)
			return RepoMetadata{}, nil
		},
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
	}

	_, err := svc.InstallTarget("template:miglite", install.Options{})
	assert.NoErr(t, err)
	assert.Eq(t, "template:miglite", store.entry.Repo)
	assert.Eq(t, "", store.entry.Tag)
	assert.Eq(t, "", store.entry.Version)
}

func TestTagFromReleaseURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "github release download", url: "https://github.com/junegunn/fzf/releases/download/v1.0.0/fzf.tar.gz", want: "v1.0.0"},
		{name: "github release download with slash tag", url: "https://github.com/windirstat/windirstat/releases/download/release/v2.6.0/windirstat.exe", want: "release/v2.6.0"},
		{name: "gitlab release asset", url: "https://gitlab.com/fdroid/fdroidserver/-/releases/v2.3.3/downloads/fdroidserver-linux-amd64.tar.gz", want: "v2.3.3"},
		{name: "gitea release download", url: "https://codeberg.org/forgejo/forgejo/releases/download/v9.0.0/forgejo-9.0.0-linux-amd64.xz", want: "v9.0.0"},
		{name: "not release url", url: "https://gitlab.com/fdroid/fdroidserver/-/package_files/123/download", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Eq(t, tt.want, tagFromReleaseURL(tt.url))
		})
	}
}

func TestInstallTargetRecordsTemplateVersionAndRunAssetMode(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	cfg := mustLoadFromString(t, `
[packages.claude]
repo = "template:claude"
latest_url = "https://example.com/latest"
latest_format = "text"
url_template = "https://example.com/{version}/{os}-{arch}/claude{ext}"
os_map = { windows = "win32" }
arch_map = { amd64 = "x64" }
ext_map = { windows = ".exe" }
checksum_url_template = "https://example.com/{version}/manifest.json"
checksum_format = "json"
checksum_json_path = "platforms.{os}-{arch}.checksum"
install_action = "run-asset"
install_args = ["install", "latest"]
`)
	runner := &fakeRunner{
		result: RunResult{
			URL:         "https://example.com/1.2.3/win32-x64/claude.exe",
			Tool:        "claude",
			Asset:       "claude.exe",
			InstallMode: install.InstallModeRunAsset,
			Version:     "1.2.3",
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{
		Runner: runner,
		Store:  store,
		Now:    func() time.Time { return now },
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("claude", install.Options{})
	if err != nil {
		t.Fatalf("install template package: %v", err)
	}

	assert.Eq(t, "claude", store.target)
	assert.Eq(t, "template:claude", store.entry.Repo)
	assert.Eq(t, "template:claude", store.entry.Target)
	assert.Eq(t, "1.2.3", store.entry.Tag)
	assert.Eq(t, "1.2.3", store.entry.Version)
	assert.Eq(t, install.InstallModeRunAsset, store.entry.InstallMode)
	assert.Eq(t, "https://example.com/latest", store.entry.Options["latest_url"])
	assert.Eq(t, "run-asset", store.entry.Options["install_action"])
	assert.Eq(t, []string{"install", "latest"}, store.entry.Options["install_args"])
}

func TestInstallTargetRecordsManagedPackageNameAsInstalledKey(t *testing.T) {
	cfg := mustLoadFromString(t, `
[packages.greq]
repo = "gookit/greq"
asset_filters = ["greq"]

[packages.gbench]
repo = "gookit/greq"
asset_filters = ["gbench"]
`)
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/gookit/greq/releases/download/v1.0.0/greq.exe",
			ExtractedFiles: []string{"./greq.exe"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{
		Runner: runner,
		Store:  store,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("greq", install.Options{})
	if err != nil {
		t.Fatalf("install managed package: %v", err)
	}

	assert.Eq(t, "greq", store.target)
	assert.Eq(t, "gookit/greq", store.entry.Repo)
	assert.Eq(t, "gookit/greq", store.entry.Target)
}

func TestInstallTargetRecordsCLINameAsInstalledKey(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/gookit/greq/releases/download/v1.0.0/gbench.exe",
			ExtractedFiles: []string{"./gbench.exe"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{
		Runner: runner,
		Store:  store,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
	}

	_, err := svc.InstallTarget("gookit/greq", install.Options{Name: "gbench"})
	if err != nil {
		t.Fatalf("install repo with CLI name: %v", err)
	}

	assert.Eq(t, "gbench", store.target)
	assert.Eq(t, "gookit/greq", store.entry.Repo)
	assert.Eq(t, "gookit/greq", store.entry.Target)
}

func TestInstallTargetReturnsRunnerErrorWithoutRecording(t *testing.T) {
	runner := &fakeRunner{err: errors.New("boom")}
	store := &fakeInstalledStore{}
	svc := Service{Runner: runner, Store: store}

	_, err := svc.InstallTarget("junegunn/fzf", install.Options{})
	if err == nil {
		t.Fatal("expected install target to return runner error")
	}
	if store.calls != 0 {
		t.Fatalf("expected store to not be called on runner error, got %d", store.calls)
	}
}
