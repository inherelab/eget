package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	"github.com/inherelab/eget/internal/util"
)

func TestInstallTargetUsesConfiguredDefaults(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	cfg := mustLoadFromString(t, `
[global]
target = "~/.local/bin"
cache_dir = "~/.cache/eget"
proxy_url = "http://127.0.0.1:7890"
`)
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://example.com/tool.tar.gz",
			ExtractedFiles: []string{"./tool"},
		},
	}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("junegunn/fzf", install.Options{})
	if err != nil {
		t.Fatalf("install target: %v", err)
	}

	expectedTarget, err := util.Expand("~/.local/bin")
	if err != nil {
		t.Fatalf("expand target: %v", err)
	}
	expectedCache, err := util.Expand("~/.cache/eget")
	if err != nil {
		t.Fatalf("expand cache: %v", err)
	}

	if runner.opts.Output != expectedTarget {
		t.Fatalf("expected configured install target, got %q", runner.opts.Output)
	}
	if runner.opts.CacheDir != expectedCache {
		t.Fatalf("expected configured cache dir, got %q", runner.opts.CacheDir)
	}
	if runner.opts.ProxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("expected configured proxy url, got %q", runner.opts.ProxyURL)
	}
	if expected := filepath.Join(expectedCache, "api-cache"); runner.opts.APICacheDir != expected {
		t.Fatalf("expected derived api cache dir, got %q", runner.opts.APICacheDir)
	}
}

func TestInstallTargetNoProxySkipsConfiguredProxyURL(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	cfg := mustLoadFromString(t, `
[global]
proxy_url = "http://127.0.0.1:7890"
`)
	runner := &fakeRunner{}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("junegunn/fzf", install.Options{NoProxy: true})

	assert.NoErr(t, err)
	assert.Eq(t, "", runner.opts.ProxyURL)
	assert.True(t, runner.opts.NoProxy)
}

func TestInstallTargetUsesConfiguredPrerelease(t *testing.T) {
	cfg := cfgpkg.NewFile()
	cfg.Packages["aeris"] = cfgpkg.Section{
		Repo:       util.StringPtr("pkgforge/aeris"),
		Prerelease: util.BoolPtr(true),
	}
	runner := &fakeRunner{}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("aeris", install.Options{})

	assert.NoErr(t, err)
	assert.True(t, runner.opts.Prerelease)
}

func TestInstallTargetNoProxyEnvSkipsConfiguredProxyURL(t *testing.T) {
	t.Setenv("NO_PROXY", "1")
	cfg := mustLoadFromString(t, `
[global]
proxy_url = "http://127.0.0.1:7890"
`)
	runner := &fakeRunner{}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("junegunn/fzf", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, "", runner.opts.ProxyURL)
}

func TestInstallTargetUsesHTTPProxyConfig(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	cfg := cfgpkg.NewFile()
	proxyURL := "http://127.0.0.1:10801"
	cfg.HTTPProxy.URL = &proxyURL
	cfg.HTTPProxy.Exclude = []string{"mydev.com"}
	cfg.Packages["fzf"] = cfgpkg.Section{Repo: util.StringPtr("junegunn/fzf")}
	runner := &fakeRunner{}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("fzf", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, "http://127.0.0.1:10801", runner.opts.ProxyURL)
	assert.Eq(t, []string{"mydev.com"}, runner.opts.ProxyExclude)
}

func TestInstallTargetHTTPProxyEnableFalseDisablesConfiguredProxy(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	enabled := false
	proxyURL := "http://127.0.0.1:10801"
	cfg := cfgpkg.NewFile()
	cfg.HTTPProxy.Enable = &enabled
	cfg.HTTPProxy.URL = &proxyURL
	runner := &fakeRunner{}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("junegunn/fzf", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, "", runner.opts.ProxyURL)
}

func TestInstallTargetHTTPProxyFallsBackToLegacyGlobalProxyURL(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	proxyURL := "http://127.0.0.1:7890"
	cfg := cfgpkg.NewFile()
	cfg.Global.ProxyURL = &proxyURL
	runner := &fakeRunner{}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("junegunn/fzf", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, proxyURL, runner.opts.ProxyURL)
}

func TestInstallTargetHTTPProxyWinsOverLegacyGlobalProxyURL(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	legacyURL := "http://127.0.0.1:7890"
	proxyURL := "http://127.0.0.1:10801"
	cfg := cfgpkg.NewFile()
	cfg.Global.ProxyURL = &legacyURL
	cfg.HTTPProxy.URL = &proxyURL
	runner := &fakeRunner{}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("junegunn/fzf", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, proxyURL, runner.opts.ProxyURL)
}

func TestInstallTargetNoProxyHostListAddsProxyExclude(t *testing.T) {
	t.Setenv("NO_PROXY", "api.github.com,*.corp.local")
	proxyURL := "http://127.0.0.1:10801"
	cfg := cfgpkg.NewFile()
	cfg.HTTPProxy.URL = &proxyURL
	cfg.HTTPProxy.Exclude = []string{"mydev.com"}
	runner := &fakeRunner{}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("junegunn/fzf", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, proxyURL, runner.opts.ProxyURL)
	assert.Eq(t, []string{"mydev.com", "api.github.com", "*.corp.local"}, runner.opts.ProxyExclude)
}

func TestInstallTargetHTTPProxyAllowsPackageRepoAndCLIProxyURLOverrides(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	globalURL := "http://127.0.0.1:10801"
	repoURL := "http://127.0.0.1:10802"
	packageURL := "http://127.0.0.1:10803"
	cliURL := "http://127.0.0.1:10804"
	cfg := cfgpkg.NewFile()
	cfg.HTTPProxy.URL = &globalURL
	cfg.Repos["junegunn/fzf"] = cfgpkg.Section{ProxyURL: &repoURL}
	cfg.Packages["fzf"] = cfgpkg.Section{
		Repo:     util.StringPtr("junegunn/fzf"),
		ProxyURL: &packageURL,
	}
	runner := &fakeRunner{}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("fzf", install.Options{})
	assert.NoErr(t, err)
	assert.Eq(t, packageURL, runner.opts.ProxyURL)

	cfg.Packages["fzf"] = cfgpkg.Section{Repo: util.StringPtr("junegunn/fzf")}
	_, err = svc.InstallTarget("fzf", install.Options{})
	assert.NoErr(t, err)
	assert.Eq(t, repoURL, runner.opts.ProxyURL)

	_, err = svc.InstallTarget("fzf", install.Options{ProxyURL: cliURL})
	assert.NoErr(t, err)
	assert.Eq(t, cliURL, runner.opts.ProxyURL)
}

func TestInstallOptionsIncludeCacheMirrorConfig(t *testing.T) {
	cacheDir := t.TempDir()
	cfg := cfgpkg.NewFile()
	cfg.Global.CacheDir = util.StringPtr(cacheDir)
	cfg.CacheMirror.Enable = util.BoolPtr(true)
	cfg.CacheMirror.URL = util.StringPtr("http://mirror.local:8686/")
	cfg.CacheMirror.Timeout = intPtr(2)
	cfg.CacheMirror.Fallback = util.BoolPtr(false)
	svc := Service{LoadConfig: func() (*cfgpkg.File, error) { return cfg, nil }}

	opts, err := svc.resolveInstallOptions("owner/repo", install.Options{}, false)

	assert.NoErr(t, err)
	assert.True(t, opts.CacheMirror.Enable)
	assert.Eq(t, "http://mirror.local:8686", opts.CacheMirror.URL)
	assert.Eq(t, 2*time.Second, opts.CacheMirror.Timeout)
	assert.False(t, opts.CacheMirror.Fallback)
}

func TestResolveInstallOptionsPreservesRetries(t *testing.T) {
	svc := Service{LoadConfig: func() (*cfgpkg.File, error) { return cfgpkg.NewFile(), nil }}

	opts, err := svc.resolveInstallOptions("owner/repo", install.Options{Retries: 3}, false)

	assert.NoErr(t, err)
	assert.Eq(t, 3, opts.Retries)
}

func TestInstallTargetUsesDefaultTargetWhenGlobalTargetMissingOrEmpty(t *testing.T) {
	tests := []struct {
		name string
		cfg  *cfgpkg.File
	}{
		{
			name: "missing",
			cfg:  cfgpkg.NewFile(),
		},
		{
			name: "empty",
			cfg: func() *cfgpkg.File {
				cfg := cfgpkg.NewFile()
				cfg.Global.Target = util.StringPtr("")
				return cfg
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{}
			svc := Service{
				Runner: runner,
				LoadConfig: func() (*cfgpkg.File, error) {
					return tt.cfg, nil
				},
			}

			_, err := svc.InstallTarget("junegunn/fzf", install.Options{})

			assert.NoErr(t, err)
			expectedTarget, err := util.Expand("~/.local/bin")
			assert.NoErr(t, err)
			assert.Eq(t, expectedTarget, runner.opts.Output)
			assert.False(t, runner.opts.OutputExplicit)
		})
	}
}

func TestInstallTargetResolvesManagedPackageName(t *testing.T) {
	cfg := mustLoadFromString(t, `
[global]
target = "~/.local/bin"

["sipeed/picoclaw"]
system = "windows/amd64"

[packages.picoclaw]
repo = "sipeed/picoclaw"
desc = "Manual PicoClaw description"
target = "D:/Program/AITools/PicoClaw"
tag = "v1.2.3"
file = "*.exe"
asset_filters = ["windows"]
`)
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/sipeed/picoclaw/releases/download/v1.2.3/picoclaw.zip",
			ExtractedFiles: []string{"./picoclaw.exe"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{
		Runner: runner,
		Store:  store,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
		RepoMetadata: func(repo string) (RepoMetadata, error) {
			return RepoMetadata{Desc: "Repository PicoClaw description"}, nil
		},
	}

	_, err := svc.InstallTarget("picoclaw", install.Options{})
	if err != nil {
		t.Fatalf("install target: %v", err)
	}

	if runner.target != "sipeed/picoclaw" {
		t.Fatalf("expected managed package to resolve repo, got %q", runner.target)
	}
	if runner.opts.Output != "D:/Program/AITools/PicoClaw" {
		t.Fatalf("expected package target to be used, got %q", runner.opts.Output)
	}
	if runner.opts.System != "windows/amd64" {
		t.Fatalf("expected repo system to be merged, got %q", runner.opts.System)
	}
	if runner.opts.Tag != "v1.2.3" {
		t.Fatalf("expected package tag to be merged, got %q", runner.opts.Tag)
	}
	if runner.opts.ExtractFile != "*.exe" {
		t.Fatalf("expected package file glob to be merged, got %q", runner.opts.ExtractFile)
	}
	if !runner.opts.All {
		t.Fatal("expected file glob to enable extract-all mode")
	}
	if len(runner.opts.Asset) != 1 || runner.opts.Asset[0] != "windows" {
		t.Fatalf("expected package asset filter to be merged, got %#v", runner.opts.Asset)
	}
	if store.target != "picoclaw" {
		t.Fatalf("expected installed store to record package name, got %q", store.target)
	}
	if store.entry.Repo != "sipeed/picoclaw" {
		t.Fatalf("expected installed repo sipeed/picoclaw, got %q", store.entry.Repo)
	}
	if store.entry.Target != "sipeed/picoclaw" {
		t.Fatalf("expected installed target to be real repo, got %q", store.entry.Target)
	}
	assert.Eq(t, "Manual PicoClaw description", store.entry.Desc)
}

func TestInstallTargetMergesTemplatePackageOptions(t *testing.T) {
	cfg := mustLoadFromString(t, `
[packages.claude]
repo = "template:claude"
latest_url = "https://example.com/latest"
latest_format = "text"
url_template = "https://example.com/{version}/{os}-{arch}/claude{ext}"
os_map = { windows = "win32" }
arch_map = { amd64 = "x64" }
ext_map = { windows = ".exe" }
install_action = "run-asset"
install_args = ["install", "latest"]
`)
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://example.com/1.2.3/win32-x64/claude.exe",
			Tool:           "claude",
			ExtractedFiles: []string{"./claude.exe"},
		},
	}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("claude", install.Options{})
	if err != nil {
		t.Fatalf("install template package: %v", err)
	}

	assert.Eq(t, "template:claude", runner.target)
	assert.Eq(t, "https://example.com/latest", runner.opts.URLTemplate.LatestURL)
	assert.Eq(t, "https://example.com/{version}/{os}-{arch}/claude{ext}", runner.opts.URLTemplate.URLTemplate)
	assert.Eq(t, map[string]string{"windows": "win32"}, runner.opts.URLTemplate.OSMap)
	assert.Eq(t, map[string]string{"amd64": "x64"}, runner.opts.URLTemplate.ArchMap)
	assert.Eq(t, map[string]string{"windows": ".exe"}, runner.opts.URLTemplate.ExtMap)
	assert.Eq(t, "run-asset", runner.opts.URLTemplate.InstallAction)
	assert.Eq(t, []string{"install", "latest"}, runner.opts.URLTemplate.InstallArgs)
}

func TestInstallTargetResolvesPkgTemplateShortAlias(t *testing.T) {
	cfg := mustLoadFromString(t, `
[pkg_templates.mydev]
latest_url = "http://mydev.lan/tools/{name}/latest.yaml"
latest_format = "yaml"
url_template = "http://mydev.lan/tools/{name}/{name}-{os}-{arch}{ext}"
os_map = { windows = "win" }
arch_map = { amd64 = "x64" }
ext_map = { windows = ".exe" }
`)
	runner := &fakeRunner{result: RunResult{URL: "http://mydev.lan/tools/markview/markview-win-x64.exe", Tool: "markview", ExtractedFiles: []string{"./markview.exe"}}}
	svc := Service{Runner: runner, LoadConfig: func() (*cfgpkg.File, error) { return cfg, nil }}

	_, err := svc.InstallTarget("mydev:markview", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, "pkg-template:mydev:markview", runner.target)
	assert.Eq(t, "http://mydev.lan/tools/markview/latest.yaml", runner.opts.URLTemplate.LatestURL)
	assert.Eq(t, "http://mydev.lan/tools/{name}/{name}-{os}-{arch}{ext}", runner.opts.URLTemplate.URLTemplate)
	assert.Eq(t, map[string]string{"windows": "win"}, runner.opts.URLTemplate.OSMap)
	assert.Eq(t, map[string]string{"amd64": "x64"}, runner.opts.URLTemplate.ArchMap)
	assert.Eq(t, map[string]string{"windows": ".exe"}, runner.opts.URLTemplate.ExtMap)
}

func TestInstallTargetResolvesConfiguredPkgTemplatePackageWithOverride(t *testing.T) {
	cfg := mustLoadFromString(t, `
[global]
target = "~/global-bin"

[pkg_templates.mydev]
latest_url = "http://mydev.lan/tools/{name}/latest.yaml"
latest_format = "yaml"
url_template = "http://mydev.lan/tools/{name}/{name}-{os}-{arch}{ext}"
os_map = { windows = "win" }
arch_map = { amd64 = "x64" }
ext_map = { windows = ".exe" }

[packages.markview]
repo = "pkg-template:mydev:markview"
desc = "Markdown preview"
target = "~/package-bin"
url_template = "http://override/{name}-{version}{ext}"
ext_map = { windows = ".zip" }
`)
	runner := &fakeRunner{result: RunResult{URL: "http://mydev.lan/tools/markview/markview-windows-amd64.zip", Tool: "markview", ExtractedFiles: []string{"./markview.exe"}}}
	svc := Service{Runner: runner, LoadConfig: func() (*cfgpkg.File, error) { return cfg, nil }}

	_, err := svc.InstallTarget("markview", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, "pkg-template:mydev:markview", runner.target)
	assert.Eq(t, "http://mydev.lan/tools/markview/latest.yaml", runner.opts.URLTemplate.LatestURL)
	assert.Eq(t, "http://override/{name}-{version}{ext}", runner.opts.URLTemplate.URLTemplate)
	assert.Eq(t, map[string]string{"windows": "win"}, runner.opts.URLTemplate.OSMap)
	assert.Eq(t, map[string]string{"amd64": "x64"}, runner.opts.URLTemplate.ArchMap)
	assert.Eq(t, map[string]string{"windows": ".zip"}, runner.opts.URLTemplate.ExtMap)
	assert.Contains(t, runner.opts.Output, "package-bin")
}

func TestInstallTargetShortAliasUsesConfiguredPkgTemplateOverride(t *testing.T) {
	cfg := mustLoadFromString(t, `
[pkg_templates.mydev]
latest_url = "http://mydev.lan/tools/{name}/latest.yaml"
url_template = "http://mydev.lan/tools/{name}/{name}-{os}-{arch}{ext}"
ext_map = { windows = ".exe" }

[packages.markview]
repo = "pkg-template:mydev:markview"
ext_map = { windows = ".zip" }
`)
	runner := &fakeRunner{result: RunResult{URL: "http://mydev.lan/tools/markview/markview-windows-amd64.zip", Tool: "markview", ExtractedFiles: []string{"./markview.exe"}}}
	svc := Service{Runner: runner, LoadConfig: func() (*cfgpkg.File, error) { return cfg, nil }}

	_, err := svc.InstallTarget("mydev:markview", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, "pkg-template:mydev:markview", runner.target)
	assert.Eq(t, map[string]string{"windows": ".zip"}, runner.opts.URLTemplate.ExtMap)
}

func TestInstallTargetShortAliasUsesCustomNamedPkgTemplatePackage(t *testing.T) {
	cfg := mustLoadFromString(t, `
[pkg_templates.mydev]
latest_url = "http://mydev.lan/tools/{name}/latest.yaml"
url_template = "http://mydev.lan/tools/{name}/{name}-{os}-{arch}{ext}"

[packages.mv]
repo = "pkg-template:mydev:markview"
target = "~/custom-bin"
`)
	runner := &fakeRunner{result: RunResult{URL: "http://mydev.lan/tools/markview/markview-windows-amd64", Tool: "markview", ExtractedFiles: []string{"./markview"}}}
	svc := Service{Runner: runner, LoadConfig: func() (*cfgpkg.File, error) { return cfg, nil }}

	_, err := svc.InstallTarget("mydev:markview", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, "pkg-template:mydev:markview", runner.target)
	assert.Contains(t, runner.opts.Output, "custom-bin")
}

func TestInstallTargetAppliesManagedPackageOptionsWhenTargetIsRepo(t *testing.T) {
	cfg := mustLoadFromString(t, `
[packages.erd]
repo = "solidiquis/erdtree"
name = "erd"
file = "erd"
asset_filters = ["musl"]
rename_files = { "erdtree" = "erd" }
strip_components = 1
`)
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/solidiquis/erdtree/releases/download/v3.1.2/erdtree.tar.gz",
			ExtractedFiles: []string{"./erd"},
		},
	}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("solidiquis/erdtree", install.Options{})
	if err != nil {
		t.Fatalf("install target: %v", err)
	}

	if runner.target != "solidiquis/erdtree" {
		t.Fatalf("expected repo target to be installed, got %q", runner.target)
	}
	if runner.opts.Name != "erd" {
		t.Fatalf("expected package name to be merged, got %q", runner.opts.Name)
	}
	if runner.opts.ExtractFile != "erd" {
		t.Fatalf("expected package file to be merged, got %q", runner.opts.ExtractFile)
	}
	if len(runner.opts.Asset) != 1 || runner.opts.Asset[0] != "musl" {
		t.Fatalf("expected package asset filter to be merged, got %#v", runner.opts.Asset)
	}
	assert.Eq(t, map[string]string{"erdtree": "erd"}, runner.opts.RenameFiles)
	assert.Eq(t, 1, runner.opts.StripComponents)
}

func TestInstallTargetWithDifferentCLINameDoesNotReuseRepoPackageOptions(t *testing.T) {
	cfg := mustLoadFromString(t, `
[packages.erd]
repo = "solidiquis/erdtree"
name = "erd"
file = "erd"
asset_filters = ["archive"]
[packages.erd.rename_files]
erdtree = "erd"
`)
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/solidiquis/erdtree/releases/download/v3.1.2/erdtree.tar.gz",
			ExtractedFiles: []string{"./erd"},
		},
	}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("solidiquis/erdtree", install.Options{
		Name:  "custom-erd",
		Asset: []string{"standalone"},
	})
	if err != nil {
		t.Fatalf("install target: %v", err)
	}

	assert.Eq(t, "custom-erd", runner.opts.Name)
	assert.Eq(t, []string{"standalone"}, runner.opts.Asset)
	assert.Eq(t, "", runner.opts.ExtractFile)
	assert.Eq(t, 0, len(runner.opts.RenameFiles))

	_, err = svc.InstallTarget("solidiquis/erdtree", install.Options{Name: "erd"})
	assert.NoErr(t, err)
	assert.Eq(t, "erd", runner.opts.ExtractFile)
	assert.Eq(t, map[string]string{"erdtree": "erd"}, runner.opts.RenameFiles)
}

func TestInstallTargetRejectsManagedPackageWithoutRepo(t *testing.T) {
	cfg := mustLoadFromString(t, `
[packages.picoclaw]
target = "D:/Program/AITools/PicoClaw"
`)
	svc := Service{
		Runner: &fakeRunner{},
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("picoclaw", install.Options{})
	if err == nil {
		t.Fatal("expected install target to fail when package repo is missing")
	}
	if err.Error() != `package "picoclaw" has no repo` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func intPtr(v int) *int {
	return &v
}
