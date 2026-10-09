package config

import (
	"path/filepath"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestLoadFileSupportsLegacyRepoSections(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	writeTestFile(t, configPath, `
[global]
target = "~/bin"
gui_target = "~/Applications"
quiet = true
github_token = "token"

["owner/repo"]
asset_filters = ["linux", "!arm"]
download_only = true
extract_all = true
strip_components = 1
is_gui = true
install_mode = "installer"
`)

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("load file: %v", err)
	}

	if cfg.Global.Target == nil || *cfg.Global.Target != "~/bin" {
		t.Fatalf("expected global target to be loaded, got %#v", cfg.Global.Target)
	}
	if cfg.Global.GuiTarget == nil || *cfg.Global.GuiTarget != "~/Applications" {
		t.Fatalf("expected global gui_target to be loaded, got %#v", cfg.Global.GuiTarget)
	}
	if cfg.Global.Quiet == nil || !*cfg.Global.Quiet {
		t.Fatalf("expected global quiet=true, got %#v", cfg.Global.Quiet)
	}

	repo, ok := cfg.Repos["owner/repo"]
	if !ok {
		t.Fatalf("expected legacy repo section to load")
	}
	if repo.DownloadOnly == nil || !*repo.DownloadOnly {
		t.Fatalf("expected repo download_only=true, got %#v", repo.DownloadOnly)
	}
	if len(repo.AssetFilters) != 2 {
		t.Fatalf("expected repo asset filters to load, got %#v", repo.AssetFilters)
	}
	if repo.ExtractAll == nil || !*repo.ExtractAll {
		t.Fatalf("expected repo extract_all=true, got %#v", repo.ExtractAll)
	}
	if repo.StripComponents == nil || *repo.StripComponents != 1 {
		t.Fatalf("expected repo strip_components=1, got %#v", repo.StripComponents)
	}
	if repo.IsGUI == nil || !*repo.IsGUI {
		t.Fatalf("expected repo is_gui=true, got %#v", repo.IsGUI)
	}
	if repo.InstallMode == nil || *repo.InstallMode != "installer" {
		t.Fatalf("expected repo install_mode=installer, got %#v", repo.InstallMode)
	}
}

func TestLoadFileInitializesPackagesMapWhenSectionMissing(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	writeTestFile(t, configPath, `
[global]
target = "~/bin"
`)

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("load file: %v", err)
	}

	if cfg.Packages == nil {
		t.Fatal("expected packages map to be initialized")
	}
}

func TestLoadFileSupportsAPICacheAndGhproxySections(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	writeTestFile(t, configPath, `
[global]
target = "~/bin"

[api_cache]
enable = true
cache_time = 300

[ghproxy]
host_url = "https://gh.felicity.ac.cn"
fallbacks = ["https://gh.llkk.cc", "https://gh.fhjhy.top"]
`)

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("load file: %v", err)
	}

	if cfg.ApiCache.Enable == nil || !*cfg.ApiCache.Enable {
		t.Fatalf("expected api_cache.enable=true, got %#v", cfg.ApiCache.Enable)
	}
	if cfg.ApiCache.CacheTime == nil || *cfg.ApiCache.CacheTime != 300 {
		t.Fatalf("expected api_cache.cache_time=300, got %#v", cfg.ApiCache.CacheTime)
	}
	if cfg.Ghproxy.HostURL == nil || *cfg.Ghproxy.HostURL != "https://gh.felicity.ac.cn" {
		t.Fatalf("expected ghproxy.host_url, got %#v", cfg.Ghproxy.HostURL)
	}
	if len(cfg.Ghproxy.Fallbacks) != 2 {
		t.Fatalf("expected ghproxy fallbacks to load, got %#v", cfg.Ghproxy.Fallbacks)
	}
	if _, ok := cfg.Repos["api_cache"]; ok {
		t.Fatalf("expected api_cache to not be treated as repo section")
	}
	if _, ok := cfg.Repos["ghproxy"]; ok {
		t.Fatalf("expected ghproxy to not be treated as repo section")
	}
}

func TestLoadFileSupportsCacheMirrorSection(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	writeTestFile(t, configPath, `
[cache_mirror]
enable = true
url = "http://mirror.local:8686"
timeout = 3
fallback = false
`)

	cfg, err := LoadFile(configPath)

	assert.NoErr(t, err)
	assert.True(t, cfg.CacheMirror.Enable != nil && *cfg.CacheMirror.Enable)
	assert.Eq(t, "http://mirror.local:8686", *cfg.CacheMirror.URL)
	assert.Eq(t, 3, *cfg.CacheMirror.Timeout)
	assert.True(t, cfg.CacheMirror.Fallback != nil && !*cfg.CacheMirror.Fallback)
	if _, ok := cfg.Repos["cache_mirror"]; ok {
		t.Fatalf("expected cache_mirror to not be treated as repo section")
	}
}

func TestLoadFileDecodesConcurrencyOptions(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	writeTestFile(t, configPath, `
[global]
chunk_concurrency = 0
batch_concurrency = 3

[packages.fd]
repo = "sharkdp/fd"
chunk_concurrency = 2

["sharkdp/fd"]
chunk_concurrency = 4
`)

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Global.ChunkConcurrency == nil || *cfg.Global.ChunkConcurrency != 0 {
		t.Fatalf("expected global chunk_concurrency=0, got %#v", cfg.Global.ChunkConcurrency)
	}
	if cfg.Global.BatchConcurrency == nil || *cfg.Global.BatchConcurrency != 3 {
		t.Fatalf("expected global batch_concurrency=3, got %#v", cfg.Global.BatchConcurrency)
	}
	if cfg.Packages["fd"].ChunkConcurrency == nil || *cfg.Packages["fd"].ChunkConcurrency != 2 {
		t.Fatalf("expected package chunk_concurrency=2, got %#v", cfg.Packages["fd"].ChunkConcurrency)
	}
	if cfg.Repos["sharkdp/fd"].ChunkConcurrency == nil || *cfg.Repos["sharkdp/fd"].ChunkConcurrency != 4 {
		t.Fatalf("expected repo chunk_concurrency=4, got %#v", cfg.Repos["sharkdp/fd"].ChunkConcurrency)
	}
}

func TestLoadFileReadsGlobalSys7zPath(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	writeTestFile(t, configPath, `
[global]
sys7z_path = "C:/Program Files/7-Zip/7z.exe"
`)

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	assert.Eq(t, "C:/Program Files/7-Zip/7z.exe", *cfg.Global.Sys7zPath)
}

func TestLoadFileReadsGlobalIgnoreUpdatePackages(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	writeTestFile(t, configPath, `
[global]
ignore_update_packages = ["fzf", "rg"]
`)

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	assert.Eq(t, []string{"fzf", "rg"}, cfg.Global.IgnoreUpdatePackages)
}

func TestLoadFileReadsPackageIgnoreUpdate(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	writeTestFile(t, configPath, `
[packages.vault]
repo = "https://releases.hashicorp.com/vault/2.1.1/vault_2.1.1_linux_amd64.zip"
ignore_update = true

[packages.rg]
repo = "BurntSushi/ripgrep"
`)

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	assert.True(t, *cfg.Packages["vault"].IgnoreUpdate)
	assert.Nil(t, cfg.Packages["rg"].IgnoreUpdate)
}

func TestLoadFileReadsSDKSections(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	writeTestFile(t, configPath, `
[global]
sdk_target = "~/sdks"
sdk_ext_map = { windows = "zip", linux = "tar.gz" }
user_agent = "custom-agent/1.0"

[sdk.go]
aliases = ["golang"]
target = "gosdk/go{version}"
url_template = "https://example.com/go{version}.{os}-{arch}.{ext}"
index_url = "https://example.com/golang/"
index_format = "html"
index_parser = "go-json"
index_path_prefix = "/golang/"
filename_pattern = "go{version}.{os}-{arch}.{ext}"
strip_components = 1
os_map = { windows = "windows", linux = "linux" }
arch_map = { amd64 = "amd64", arm64 = "arm64" }
ext_map = { windows = "zip", linux = "tar.gz" }
`)

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Global.SDKTarget == nil || *cfg.Global.SDKTarget != "~/sdks" {
		t.Fatalf("expected global.sdk_target, got %#v", cfg.Global.SDKTarget)
	}
	assert.Eq(t, "tar.gz", cfg.Global.SDKExtMap["linux"])
	assert.Eq(t, "custom-agent/1.0", *cfg.Global.UserAgent)
	got := cfg.SDK["go"]
	assert.Eq(t, []string{"golang"}, got.Aliases)
	assert.Eq(t, "gosdk/go{version}", *got.Target)
	assert.Eq(t, "https://example.com/go{version}.{os}-{arch}.{ext}", *got.URLTemplate)
	assert.Eq(t, "https://example.com/golang/", *got.IndexURL)
	assert.Eq(t, "html", *got.IndexFormat)
	assert.Eq(t, "go-json", *got.IndexParser)
	assert.Eq(t, "/golang/", *got.IndexPathPrefix)
	assert.Eq(t, "go{version}.{os}-{arch}.{ext}", *got.FilenamePattern)
	assert.Eq(t, 1, *got.StripComponents)
	assert.Eq(t, "linux", got.OSMap["linux"])
	assert.Eq(t, "arm64", got.ArchMap["arm64"])
	assert.Eq(t, "zip", got.ExtMap["windows"])
}
