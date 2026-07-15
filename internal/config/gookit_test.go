package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestLoadFileTracksGlobalSection(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{name: "missing", body: "[packages.fzf]\nrepo = 'junegunn/fzf'\n"},
		{name: "present", body: "[global]\n\n[packages.fzf]\nrepo = 'junegunn/fzf'\n", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "eget.toml")
			assert.NoErr(t, os.WriteFile(path, []byte(tc.body), 0o644))

			cfg, err := LoadFile(path)

			assert.NoErr(t, err)
			assert.Eq(t, tc.want, cfg.Meta.HasGlobal)
		})
	}
}

func TestDumpExportOmitsGlobalByDefault(t *testing.T) {
	target, repo := "/machine/bin", "junegunn/fzf"
	cfg := NewFile()
	cfg.Global.Target = &target
	cfg.Packages["fzf"] = Section{Repo: &repo}

	var out bytes.Buffer
	assert.NoErr(t, DumpExport(&out, cfg, false))
	assert.NotContains(t, out.String(), "[global]")
	assert.Contains(t, out.String(), "[packages.fzf]")

	out.Reset()
	assert.NoErr(t, DumpExport(&out, cfg, true))
	assert.Contains(t, out.String(), "[global]")
	assert.Contains(t, out.String(), "/machine/bin")
}

func TestSaveAtomic(t *testing.T) {
	t.Run("writes a loadable replacement", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "eget.toml")
		target := "/new/bin"
		cfg := NewFile()
		cfg.Global.Target = &target

		assert.NoErr(t, SaveAtomic(path, cfg))
		loaded, err := LoadFile(path)
		assert.NoErr(t, err)
		assert.Eq(t, target, *loaded.Global.Target)
	})

	t.Run("keeps old file when replacement fails", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "eget.toml")
		assert.NoErr(t, os.WriteFile(path, []byte("old"), 0o644))
		oldReplace := replaceConfigFile
		replaceConfigFile = func(string, string) error { return errors.New("replace failed") }
		defer func() { replaceConfigFile = oldReplace }()

		err := SaveAtomic(path, NewFile())

		assert.Err(t, err)
		body, readErr := os.ReadFile(path)
		assert.NoErr(t, readErr)
		assert.Eq(t, "old", string(body))
		files, globErr := filepath.Glob(filepath.Join(dir, ".eget.toml.tmp-*"))
		assert.NoErr(t, globErr)
		assert.Len(t, files, 0)
	})
}

func TestPathGetAndSet(t *testing.T) {
	cfg := NewFile()

	if err := SetByPath(cfg, "global.target", "~/.local/bin"); err != nil {
		t.Fatalf("set global.target: %v", err)
	}
	if err := SetByPath(cfg, "global.gui_target", "~/Applications"); err != nil {
		t.Fatalf("set global.gui_target: %v", err)
	}
	if err := SetByPath(cfg, "api_cache.enable", "true"); err != nil {
		t.Fatalf("set api_cache.enable: %v", err)
	}
	if err := SetByPath(cfg, "api_cache.cache_time", "300"); err != nil {
		t.Fatalf("set api_cache.cache_time: %v", err)
	}
	if err := SetByPath(cfg, "cache_mirror.enable", "true"); err != nil {
		t.Fatalf("set cache_mirror.enable: %v", err)
	}
	if err := SetByPath(cfg, "cache_mirror.url", "http://mirror.local:8686"); err != nil {
		t.Fatalf("set cache_mirror.url: %v", err)
	}
	if err := SetByPath(cfg, "cache_mirror.timeout", "3"); err != nil {
		t.Fatalf("set cache_mirror.timeout: %v", err)
	}
	if err := SetByPath(cfg, "cache_mirror.fallback", "false"); err != nil {
		t.Fatalf("set cache_mirror.fallback: %v", err)
	}
	if err := SetByPath(cfg, "global.chunk_concurrency", "0"); err != nil {
		t.Fatalf("set global.chunk_concurrency: %v", err)
	}
	if err := SetByPath(cfg, "global.batch_concurrency", "3"); err != nil {
		t.Fatalf("set global.batch_concurrency: %v", err)
	}
	if err := SetByPath(cfg, "global.user_agent", "custom-agent/1.0"); err != nil {
		t.Fatalf("set global.user_agent: %v", err)
	}
	if err := SetByPath(cfg, "packages.fzf.repo", "junegunn/fzf"); err != nil {
		t.Fatalf("set packages.fzf.repo: %v", err)
	}
	if err := SetByPath(cfg, "packages.fzf.desc", "Command-line fuzzy finder"); err != nil {
		t.Fatalf("set packages.fzf.desc: %v", err)
	}
	if err := SetByPath(cfg, "packages.fzf.chunk_concurrency", "2"); err != nil {
		t.Fatalf("set packages.fzf.chunk_concurrency: %v", err)
	}
	if err := SetByPath(cfg, "packages.fzf.strip_components", "1"); err != nil {
		t.Fatalf("set packages.fzf.strip_components: %v", err)
	}
	if err := SetByPath(cfg, "packages.fzf.asset_filters", "linux,amd64"); err != nil {
		t.Fatalf("set packages.fzf.asset_filters: %v", err)
	}
	if err := SetByPath(cfg, "packages.fzf.extract_all", "true"); err != nil {
		t.Fatalf("set packages.fzf.extract_all: %v", err)
	}
	if err := SetByPath(cfg, "packages.fzf.is_gui", "true"); err != nil {
		t.Fatalf("set packages.fzf.is_gui: %v", err)
	}

	target, ok := GetByPath(cfg, "global.target")
	if !ok || target != "~/.local/bin" {
		t.Fatalf("expected global.target to be set, got %#v ok=%t", target, ok)
	}
	guiTarget, ok := GetByPath(cfg, "global.gui_target")
	if !ok || guiTarget != "~/Applications" {
		t.Fatalf("expected global.gui_target to be set, got %#v ok=%t", guiTarget, ok)
	}
	cacheTime, ok := GetByPath(cfg, "api_cache.cache_time")
	if !ok || cacheTime != 300 {
		t.Fatalf("expected api_cache.cache_time to be 300, got %#v ok=%t", cacheTime, ok)
	}
	mirrorEnable, ok := GetByPath(cfg, "cache_mirror.enable")
	if !ok || mirrorEnable != true {
		t.Fatalf("expected cache_mirror.enable to be true, got %#v ok=%t", mirrorEnable, ok)
	}
	mirrorTimeout, ok := GetByPath(cfg, "cache_mirror.timeout")
	if !ok || mirrorTimeout != 3 {
		t.Fatalf("expected cache_mirror.timeout to be 3, got %#v ok=%t", mirrorTimeout, ok)
	}
	chunk, ok := GetByPath(cfg, "global.chunk_concurrency")
	if !ok || chunk != 0 {
		t.Fatalf("expected global.chunk_concurrency to be 0, got %#v ok=%t", chunk, ok)
	}
	batch, ok := GetByPath(cfg, "global.batch_concurrency")
	if !ok || batch != 3 {
		t.Fatalf("expected global.batch_concurrency to be 3, got %#v ok=%t", batch, ok)
	}
	userAgent, ok := GetByPath(cfg, "global.user_agent")
	if !ok || userAgent != "custom-agent/1.0" {
		t.Fatalf("expected global.user_agent to be set, got %#v ok=%t", userAgent, ok)
	}
	repo, ok := GetByPath(cfg, "packages.fzf.repo")
	if !ok || repo != "junegunn/fzf" {
		t.Fatalf("expected packages.fzf.repo to be set, got %#v ok=%t", repo, ok)
	}

	pkg, ok := cfg.Packages["fzf"]
	if !ok {
		t.Fatal("expected package fzf to be created")
	}
	if len(pkg.AssetFilters) != 2 || pkg.AssetFilters[0] != "linux" || pkg.AssetFilters[1] != "amd64" {
		t.Fatalf("expected package asset filters to be parsed, got %#v", pkg.AssetFilters)
	}
	if pkg.ExtractAll == nil || !*pkg.ExtractAll {
		t.Fatalf("expected package extract_all to be parsed, got %#v", pkg.ExtractAll)
	}
	if pkg.IsGUI == nil || !*pkg.IsGUI {
		t.Fatalf("expected package is_gui to be parsed, got %#v", pkg.IsGUI)
	}
	if pkg.ChunkConcurrency == nil || *pkg.ChunkConcurrency != 2 {
		t.Fatalf("expected package chunk_concurrency to be parsed, got %#v", pkg.ChunkConcurrency)
	}
	if pkg.StripComponents == nil || *pkg.StripComponents != 1 {
		t.Fatalf("expected package strip_components to be parsed, got %#v", pkg.StripComponents)
	}
	if pkg.Desc == nil || *pkg.Desc != "Command-line fuzzy finder" {
		t.Fatalf("expected package desc to be parsed, got %#v", pkg.Desc)
	}
}

func TestDumpConfigStringIncludesCacheMirror(t *testing.T) {
	cfg := NewFile()
	cfg.CacheMirror.Enable = boolPtr(true)
	cfg.CacheMirror.URL = stringPtr("http://mirror.local:8686")
	cfg.CacheMirror.Timeout = intPtr(3)
	cfg.CacheMirror.Fallback = boolPtr(false)

	text, err := dumpConfigString(cfg)

	assert.NoErr(t, err)
	assert.Contains(t, text, "[cache_mirror]")
	assert.Contains(t, text, "enable = true")
	assert.Contains(t, text, `url = "http://mirror.local:8686"`)
	assert.Contains(t, text, "timeout = 3")
	assert.Contains(t, text, "fallback = false")
}

func TestHTTPProxySectionRoundTrip(t *testing.T) {
	proxyURL := "http://127.0.0.1:10801"
	enabled := true
	cfg := NewFile()
	cfg.HTTPProxy.URL = &proxyURL
	cfg.HTTPProxy.Enable = &enabled
	cfg.HTTPProxy.Exclude = []string{"mydev.com", "*.corp.local", "10.0.0.0/8"}

	dumped, err := dumpConfigString(cfg)

	assert.NoErr(t, err)
	assert.Contains(t, dumped, "[http_proxy]")
	assert.Contains(t, dumped, `url = "http://127.0.0.1:10801"`)
	assert.Contains(t, dumped, "enable = true")
	assert.Contains(t, dumped, "exclude")

	loaded := newConfigManager()
	assert.NoErr(t, loaded.Config().LoadStrings("toml", dumped))
	decoded, err := decodeConfigFile(loaded)
	assert.NoErr(t, err)
	assert.Eq(t, proxyURL, *decoded.HTTPProxy.URL)
	assert.True(t, *decoded.HTTPProxy.Enable)
	assert.Eq(t, []string{"mydev.com", "*.corp.local", "10.0.0.0/8"}, decoded.HTTPProxy.Exclude)
}

func TestDecodeAndBindStruct(t *testing.T) {
	cfg := NewFile()
	target := "~/.local/bin"
	proxyURL := "http://127.0.0.1:7890"
	repo := "junegunn/fzf"
	cfg.Global.Target = &target
	cfg.Global.ProxyURL = &proxyURL
	cfg.Packages["fzf"] = Section{Repo: &repo}

	var decoded struct {
		Global struct {
			Target   string `mapstructure:"target"`
			ProxyURL string `mapstructure:"proxy_url"`
		} `mapstructure:"global"`
	}
	if err := DecodeTo(cfg, &decoded); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if decoded.Global.Target != "~/.local/bin" || decoded.Global.ProxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("unexpected decoded global config: %#v", decoded.Global)
	}

	var pkg struct {
		Repo string `mapstructure:"repo"`
	}
	if err := BindStruct(cfg, "packages.fzf", &pkg); err != nil {
		t.Fatalf("bind package struct: %v", err)
	}
	if pkg.Repo != "junegunn/fzf" {
		t.Fatalf("expected bound repo junegunn/fzf, got %q", pkg.Repo)
	}
}

func TestDumpConfigStringKeepsLegacyRepoSections(t *testing.T) {
	cfg := NewFile()
	target := "~/.local/bin"
	repoTarget := "~/repo-bin"
	repoSystem := "linux/amd64"
	repo := "junegunn/fzf"
	cfg.Global.Target = &target
	cfg.Repos["owner/repo"] = Section{
		Target:      &repoTarget,
		System:      &repoSystem,
		ExtractAll:  boolPtr(true),
		IsGUI:       boolPtr(true),
		InstallMode: stringPtr("installer"),
	}
	cfg.Packages["fzf"] = Section{Repo: &repo}

	text, err := dumpConfigString(cfg)
	if err != nil {
		t.Fatalf("dump config string: %v", err)
	}
	if !strings.Contains(text, "[\"owner/repo\"]") {
		t.Fatalf("expected quoted legacy repo section, got %q", text)
	}
	if !strings.Contains(text, "[packages.fzf]") {
		t.Fatalf("expected packages.fzf section, got %q", text)
	}
	if !strings.Contains(text, "extract_all = true") {
		t.Fatalf("expected extract_all field, got %q", text)
	}
	if !strings.Contains(text, "is_gui = true") {
		t.Fatalf("expected is_gui field, got %q", text)
	}
	if !strings.Contains(text, `install_mode = "installer"`) {
		t.Fatalf("expected install_mode field, got %q", text)
	}
	if strings.Contains(text, "\n  all = true") || strings.Contains(text, "\n    all = true") {
		t.Fatalf("expected old all field to be absent, got %q", text)
	}

	var buf bytes.Buffer
	if err := dumpConfig(cfg, &buf); err != nil {
		t.Fatalf("dump config: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected dump config to write data")
	}
}

func TestDumpConfigStringOmitsEmptySourcePath(t *testing.T) {
	cfg := NewFile()
	repo := "junegunn/fzf"
	cfg.Packages["fzf"] = Section{
		Repo:       &repo,
		SourcePath: stringPtr(""),
	}

	text, err := dumpConfigString(cfg)
	if err != nil {
		t.Fatalf("dump config string: %v", err)
	}
	if strings.Contains(text, "source_path") {
		t.Fatalf("expected empty source_path field to be absent, got %q", text)
	}
}

func TestDumpConfigStringIncludesConcurrencyOptions(t *testing.T) {
	cfg := NewFile()
	chunk := 0
	batch := 3
	pkgChunk := 2
	cfg.Global.ChunkConcurrency = &chunk
	cfg.Global.BatchConcurrency = &batch
	cfg.Packages["fd"] = Section{
		Repo:             stringPtr("sharkdp/fd"),
		Desc:             stringPtr("Simple, fast and user-friendly alternative to find"),
		ChunkConcurrency: &pkgChunk,
	}

	text, err := dumpConfigString(cfg)
	if err != nil {
		t.Fatalf("dump config string: %v", err)
	}
	if !strings.Contains(text, "chunk_concurrency = 0") {
		t.Fatalf("expected global chunk_concurrency in dump, got %q", text)
	}
	if !strings.Contains(text, "batch_concurrency = 3") {
		t.Fatalf("expected global batch_concurrency in dump, got %q", text)
	}
	if !strings.Contains(text, "chunk_concurrency = 2") {
		t.Fatalf("expected package chunk_concurrency in dump, got %q", text)
	}
	if !strings.Contains(text, `desc = "Simple, fast and user-friendly alternative to find"`) {
		t.Fatalf("expected package desc in dump, got %q", text)
	}
}

func TestDumpConfigStringIncludesSys7zPath(t *testing.T) {
	path := "C:/Program Files/7-Zip/7z.exe"
	cfg := NewFile()
	cfg.Global.Sys7zPath = &path

	text, err := dumpConfigString(cfg)
	if err != nil {
		t.Fatalf("dump config string: %v", err)
	}

	assert.Contains(t, text, `sys7z_path = "C:/Program Files/7-Zip/7z.exe"`)
}

func TestDumpConfigStringIncludesUserAgent(t *testing.T) {
	userAgent := "custom-agent/1.0"
	cfg := NewFile()
	cfg.Global.UserAgent = &userAgent

	text, err := dumpConfigString(cfg)
	if err != nil {
		t.Fatalf("dump config string: %v", err)
	}

	assert.Contains(t, text, `user_agent = "custom-agent/1.0"`)
}

func TestDumpConfigStringIncludesIgnoreUpdatePackages(t *testing.T) {
	cfg := NewFile()
	cfg.Global.IgnoreUpdatePackages = []string{"fzf", "rg"}

	text, err := dumpConfigString(cfg)
	if err != nil {
		t.Fatalf("dump config string: %v", err)
	}

	assert.Contains(t, text, `ignore_update_packages = ["fzf", "rg"]`)
}

func TestSetByPathSupportsGlobalIgnoreUpdatePackages(t *testing.T) {
	cfg := NewFile()

	if err := SetByPath(cfg, "global.ignore_update_packages", "fzf, rg"); err != nil {
		t.Fatalf("set global.ignore_update_packages: %v", err)
	}

	value, ok := GetByPath(cfg, "global.ignore_update_packages")
	if !ok {
		t.Fatal("expected global.ignore_update_packages to be set")
	}
	assert.Eq(t, []string{"fzf", "rg"}, value)
	assert.Eq(t, []string{"fzf", "rg"}, cfg.Global.IgnoreUpdatePackages)
}

func TestSetByPathSupportsGlobalSys7zPath(t *testing.T) {
	cfg := NewFile()

	if err := SetByPath(cfg, "global.sys7z_path", "C:/Tools/7z.exe"); err != nil {
		t.Fatalf("set global.sys7z_path: %v", err)
	}

	value, ok := GetByPath(cfg, "global.sys7z_path")
	if !ok {
		t.Fatal("expected global.sys7z_path to be set")
	}
	assert.Eq(t, "C:/Tools/7z.exe", value)
}

func TestSetByPathHTTPProxyFields(t *testing.T) {
	cfg := NewFile()

	assert.NoErr(t, SetByPath(cfg, "http_proxy.url", "127.0.0.1:10801"))
	assert.Eq(t, "http://127.0.0.1:10801", *cfg.HTTPProxy.URL)

	assert.NoErr(t, SetByPath(cfg, "http_proxy.enable", "false"))
	assert.False(t, *cfg.HTTPProxy.Enable)

	assert.NoErr(t, SetByPath(cfg, "http_proxy.exclude", "mydev.com,*.corp.local"))
	assert.Eq(t, []string{"mydev.com", "*.corp.local"}, cfg.HTTPProxy.Exclude)
}

func TestPackageURLTemplateFieldsRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")
	repo := "template:claude"
	latestURL := "https://downloads.claude.ai/claude-code-releases/latest"
	latestFormat := "text"
	urlTemplate := "https://downloads.claude.ai/claude-code-releases/{version}/{os}-{arch}{libc}/claude{ext}"
	checksumURL := "https://downloads.claude.ai/claude-code-releases/{version}/manifest.json"
	checksumFormat := "json"
	checksumPath := "platforms.{os}-{arch}{libc}.checksum"
	installAction := "run-asset"

	cfg := NewFile()
	cfg.Packages["claude"] = Section{
		Repo:                &repo,
		LatestURL:           &latestURL,
		LatestFormat:        &latestFormat,
		URLTemplate:         &urlTemplate,
		OSMap:               map[string]string{"windows": "win32", "linux": "linux", "darwin": "darwin"},
		ArchMap:             map[string]string{"amd64": "x64", "arm64": "arm64"},
		ExtMap:              map[string]string{"windows": ".exe", "linux": "", "darwin": ""},
		LibcMap:             map[string]string{"glibc": "", "musl": "-musl"},
		ChecksumURLTemplate: &checksumURL,
		ChecksumFormat:      &checksumFormat,
		ChecksumJSONPath:    &checksumPath,
		InstallAction:       &installAction,
		InstallArgs:         []string{"install", "latest"},
	}

	text, err := dumpConfigString(cfg)
	assert.NoErr(t, err)
	assert.Contains(t, text, `repo = "template:claude"`)
	assert.Contains(t, text, `install_args = ["install", "latest"]`)

	err = Save(configPath, cfg)
	assert.NoErr(t, err)
	loaded, err := LoadFile(configPath)
	assert.NoErr(t, err)

	pkg := loaded.Packages["claude"]
	assert.Eq(t, repo, *pkg.Repo)
	assert.Eq(t, latestURL, *pkg.LatestURL)
	assert.Eq(t, urlTemplate, *pkg.URLTemplate)
	assert.Eq(t, map[string]string{"windows": ".exe", "linux": "", "darwin": ""}, pkg.ExtMap)
	assert.Eq(t, map[string]string{"glibc": "", "musl": "-musl"}, pkg.LibcMap)
	assert.Eq(t, []string{"install", "latest"}, pkg.InstallArgs)
}

func TestPkgTemplatesSectionRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")
	latestURL := "http://mydev.lan/tools/{name}/latest.yaml"
	latestFormat := "yaml"
	urlTemplate := "http://mydev.lan/tools/{name}/{name}-{os}-{arch}{ext}"
	checksumURL := "http://mydev.lan/tools/{name}/{version}/checksums.json"
	checksumFormat := "json"
	checksumPath := "platforms.{os}-{arch}.checksum"

	cfg := NewFile()
	cfg.PkgTemplates["mydev"] = Section{
		LatestURL:           &latestURL,
		LatestFormat:        &latestFormat,
		URLTemplate:         &urlTemplate,
		OSMap:               map[string]string{"windows": "win", "linux": "linux"},
		ArchMap:             map[string]string{"amd64": "x64"},
		ExtMap:              map[string]string{"windows": ".exe", "linux": ""},
		ChecksumURLTemplate: &checksumURL,
		ChecksumFormat:      &checksumFormat,
		ChecksumJSONPath:    &checksumPath,
	}

	text, err := dumpConfigString(cfg)
	assert.NoErr(t, err)
	assert.Contains(t, text, "[pkg_templates.mydev]")
	assert.Contains(t, text, `latest_url = "http://mydev.lan/tools/{name}/latest.yaml"`)

	assert.NoErr(t, Save(configPath, cfg))
	loaded, err := LoadFile(configPath)
	assert.NoErr(t, err)
	template := loaded.PkgTemplates["mydev"]
	assert.Eq(t, latestURL, *template.LatestURL)
	assert.Eq(t, latestFormat, *template.LatestFormat)
	assert.Eq(t, urlTemplate, *template.URLTemplate)
	assert.Eq(t, map[string]string{"windows": "win", "linux": "linux"}, template.OSMap)
	assert.Eq(t, map[string]string{"amd64": "x64"}, template.ArchMap)
	assert.Eq(t, map[string]string{"windows": ".exe", "linux": ""}, template.ExtMap)
	assert.Eq(t, checksumURL, *template.ChecksumURLTemplate)
	assert.Eq(t, checksumFormat, *template.ChecksumFormat)
	assert.Eq(t, checksumPath, *template.ChecksumJSONPath)
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	cfg := NewFile()
	target := "~/.local/bin"
	sdkTarget := "~/sdks"
	userAgent := "custom-agent/1.0"
	quiet := true
	cacheTime := 300
	repo := "junegunn/fzf"
	sdkStrip := 1
	cfg.Global.Target = &target
	cfg.Global.SDKTarget = &sdkTarget
	cfg.Global.UserAgent = &userAgent
	cfg.Global.SDKExtMap = map[string]string{"windows": "zip", "linux": "tar.gz"}
	cfg.Global.Quiet = &quiet
	cfg.ApiCache.CacheTime = &cacheTime
	cfg.Packages["fzf"] = Section{Repo: &repo}
	cfg.SDK["go"] = SDKSection{
		Aliases:         []string{"golang"},
		Target:          stringPtr("gosdk/go{version}"),
		URLTemplate:     stringPtr("https://example.com/go{version}.{os}-{arch}.{ext}"),
		IndexURL:        stringPtr("https://example.com/golang/"),
		IndexFormat:     stringPtr("html"),
		IndexParser:     stringPtr("go-json"),
		IndexPathPrefix: stringPtr("/golang/"),
		FilenamePattern: stringPtr("go{version}.{os}-{arch}.{ext}"),
		StripComponents: &sdkStrip,
		OSMap:           map[string]string{"linux": "linux"},
		ArchMap:         map[string]string{"amd64": "amd64"},
		ExtMap:          map[string]string{"linux": "tar.gz"},
	}

	if err := Save(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.Global.Target == nil || *loaded.Global.Target != "~/.local/bin" {
		t.Fatalf("expected round-trip global.target, got %#v", loaded.Global.Target)
	}
	if loaded.Global.SDKTarget == nil || *loaded.Global.SDKTarget != "~/sdks" {
		t.Fatalf("expected round-trip global.sdk_target, got %#v", loaded.Global.SDKTarget)
	}
	if loaded.Global.UserAgent == nil || *loaded.Global.UserAgent != "custom-agent/1.0" {
		t.Fatalf("expected round-trip global.user_agent, got %#v", loaded.Global.UserAgent)
	}
	assert.Eq(t, "tar.gz", loaded.Global.SDKExtMap["linux"])
	if loaded.Global.Quiet == nil || !*loaded.Global.Quiet {
		t.Fatalf("expected round-trip global.quiet, got %#v", loaded.Global.Quiet)
	}
	if loaded.ApiCache.CacheTime == nil || *loaded.ApiCache.CacheTime != 300 {
		t.Fatalf("expected round-trip api_cache.cache_time, got %#v", loaded.ApiCache.CacheTime)
	}
	if loaded.Packages["fzf"].Repo == nil || *loaded.Packages["fzf"].Repo != "junegunn/fzf" {
		t.Fatalf("expected round-trip packages.fzf.repo, got %#v", loaded.Packages["fzf"].Repo)
	}
	sdk := loaded.SDK["go"]
	assert.Eq(t, []string{"golang"}, sdk.Aliases)
	assert.Eq(t, "gosdk/go{version}", *sdk.Target)
	assert.Eq(t, "go-json", *sdk.IndexParser)
	assert.Eq(t, "go{version}.{os}-{arch}.{ext}", *sdk.FilenamePattern)
	assert.Eq(t, 1, *sdk.StripComponents)
	assert.Eq(t, "tar.gz", sdk.ExtMap["linux"])
}
