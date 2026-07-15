package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
)

func TestConfigExport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "eget.toml")
	writeConfigFile(t, path, "[global]\ntarget = '/machine/bin'\n\n[packages.fzf]\nrepo = 'junegunn/fzf'\n")
	svc := ConfigService{ConfigPath: path, Load: func() (*cfgpkg.File, error) {
		return cfgpkg.LoadFile(path)
	}}

	t.Run("omits global by default", func(t *testing.T) {
		var out bytes.Buffer
		assert.NoErr(t, svc.ConfigExport(&out, false))
		assert.NotContains(t, out.String(), "[global]")
		assert.Contains(t, out.String(), "[packages.fzf]")
	})

	t.Run("includes global when requested", func(t *testing.T) {
		var out bytes.Buffer
		assert.NoErr(t, svc.ConfigExport(&out, true))
		assert.Contains(t, out.String(), "[global]")
		assert.Contains(t, out.String(), "/machine/bin")
	})
}

func TestConfigImport(t *testing.T) {
	t.Run("imports portable config when target does not exist", func(t *testing.T) {
		dir := t.TempDir()
		targetPath := filepath.Join(dir, "eget.toml")
		sourcePath := filepath.Join(dir, "portable.toml")
		writeRawConfigFile(t, sourcePath, "[packages.fzf]\nrepo = 'junegunn/fzf'\n")

		_, err := (ConfigService{ConfigPath: targetPath}).ConfigImport(sourcePath)

		assert.NoErr(t, err)
		loaded, loadErr := cfgpkg.LoadFile(targetPath)
		assert.NoErr(t, loadErr)
		assert.Eq(t, "junegunn/fzf", *loaded.Packages["fzf"].Repo)
	})

	t.Run("retains current global when source omits it", func(t *testing.T) {
		dir := t.TempDir()
		targetPath := filepath.Join(dir, "eget.toml")
		sourcePath := filepath.Join(dir, "portable.toml")
		writeConfigFile(t, targetPath, "[global]\ntarget = '/current/bin'\n\n[packages.old]\nrepo = 'owner/old'\n")
		writeRawConfigFile(t, sourcePath, "[packages.fzf]\nrepo = 'junegunn/fzf'\n")
		current, currentErr := cfgpkg.LoadFile(targetPath)
		assert.NoErr(t, currentErr)
		assert.Eq(t, "/current/bin", *current.Global.Target)
		incoming, incomingErr := cfgpkg.LoadFile(sourcePath)
		assert.NoErr(t, incomingErr)
		assert.False(t, incoming.Meta.HasGlobal)

		gotPath, err := (ConfigService{ConfigPath: targetPath}).ConfigImport(sourcePath)

		assert.NoErr(t, err)
		assert.Eq(t, targetPath, gotPath)
		loaded, loadErr := cfgpkg.LoadFile(targetPath)
		assert.NoErr(t, loadErr)
		assert.Eq(t, "/current/bin", *loaded.Global.Target)
		_, hasOld := loaded.Packages["old"]
		assert.False(t, hasOld)
		assert.Eq(t, "junegunn/fzf", *loaded.Packages["fzf"].Repo)
	})

	t.Run("replaces current global when source includes it", func(t *testing.T) {
		dir := t.TempDir()
		targetPath := filepath.Join(dir, "eget.toml")
		sourcePath := filepath.Join(dir, "complete.toml")
		writeConfigFile(t, targetPath, "[global]\ntarget = '/current/bin'\n")
		writeRawConfigFile(t, sourcePath, "[global]\ntarget = '/imported/bin'\n\n[packages.rg]\nrepo = 'BurntSushi/ripgrep'\n")

		_, err := (ConfigService{ConfigPath: targetPath}).ConfigImport(sourcePath)

		assert.NoErr(t, err)
		loaded, loadErr := cfgpkg.LoadFile(targetPath)
		assert.NoErr(t, loadErr)
		assert.Eq(t, "/imported/bin", *loaded.Global.Target)
	})

	t.Run("malformed source keeps target unchanged", func(t *testing.T) {
		dir := t.TempDir()
		targetPath := filepath.Join(dir, "eget.toml")
		sourcePath := filepath.Join(dir, "broken.toml")
		original := "[global]\ntarget = '/current/bin'\n"
		writeConfigFile(t, targetPath, original)
		writeRawConfigFile(t, sourcePath, "[global\ntarget =")
		before, beforeErr := os.ReadFile(targetPath)
		assert.NoErr(t, beforeErr)

		_, err := (ConfigService{ConfigPath: targetPath}).ConfigImport(sourcePath)

		assert.Err(t, err)
		body, readErr := os.ReadFile(targetPath)
		assert.NoErr(t, readErr)
		assert.Eq(t, string(before), string(body))
	})
}

func TestConfigInfo(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")
	writeConfigFile(t, configPath, "[global]\ntarget = \"~/bin\"\n")

	svc := ConfigService{ConfigPath: configPath}
	info, err := svc.ConfigInfo()
	if err != nil {
		t.Fatalf("config info: %v", err)
	}
	if !info.Exists {
		t.Fatal("expected config file to exist")
	}
	if info.Path != configPath {
		t.Fatalf("expected config path %q, got %q", configPath, info.Path)
	}
}

func TestConfigInit(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	svc := ConfigService{ConfigPath: configPath}
	gotPath, err := svc.ConfigInit()
	if err != nil {
		t.Fatalf("config init: %v", err)
	}
	if gotPath != configPath {
		t.Fatalf("expected init path %q, got %q", configPath, gotPath)
	}

	cfg, err := cfgpkg.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load init config: %v", err)
	}
	if cfg.Global.Target == nil || *cfg.Global.Target != "~/.local/bin" {
		t.Fatalf("expected default global.target, got %#v", cfg.Global.Target)
	}
	if cfg.Global.SDKTarget == nil || *cfg.Global.SDKTarget != "~/.local/sdks" {
		t.Fatalf("expected default global.sdk_target, got %#v", cfg.Global.SDKTarget)
	}
	if cfg.Global.System == nil || *cfg.Global.System != "" {
		t.Fatalf("expected empty global.system, got %#v", cfg.Global.System)
	}
	if cfg.Global.CacheDir == nil || *cfg.Global.CacheDir != "~/.cache/eget" {
		t.Fatalf("expected default global.cache_dir, got %#v", cfg.Global.CacheDir)
	}
	if cfg.Global.ProxyURL == nil || *cfg.Global.ProxyURL != "" {
		t.Fatalf("expected default global.proxy_url, got %#v", cfg.Global.ProxyURL)
	}
	if cfg.HTTPProxy.URL != nil && *cfg.HTTPProxy.URL != "" {
		t.Fatalf("expected default http_proxy.url to be empty, got %#v", cfg.HTTPProxy.URL)
	}
	if cfg.Global.UserAgent == nil || *cfg.Global.UserAgent == "" {
		t.Fatalf("expected default global.user_agent, got %#v", cfg.Global.UserAgent)
	}
	if cfg.Global.Sys7zPath == nil || *cfg.Global.Sys7zPath != "" {
		t.Fatalf("expected default global.sys7z_path, got %#v", cfg.Global.Sys7zPath)
	}
	if cfg.ApiCache.Enable == nil || *cfg.ApiCache.Enable {
		t.Fatalf("expected default api_cache.enable=false, got %#v", cfg.ApiCache.Enable)
	}
	if cfg.ApiCache.CacheTime == nil || *cfg.ApiCache.CacheTime != 1800 {
		t.Fatalf("expected default api_cache.cache_time=1800, got %#v", cfg.ApiCache.CacheTime)
	}
	if cfg.Ghproxy.HostURL == nil || *cfg.Ghproxy.HostURL != "" {
		t.Fatalf("expected default ghproxy.host_url, got %#v", cfg.Ghproxy.HostURL)
	}
	if len(cfg.Ghproxy.Fallbacks) != 0 {
		t.Fatalf("expected default ghproxy fallbacks, got %#v", cfg.Ghproxy.Fallbacks)
	}
	if cfg.Packages == nil {
		t.Fatal("expected packages section to be initialized")
	}
}

func TestConfigListGetAndSet(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")
	writeConfigFile(t, configPath, `
[global]
target = "~/bin"

[packages.fzf]
repo = "junegunn/fzf"
target = "~/.local/bin"
`)

	svc := ConfigService{
		ConfigPath: configPath,
		Load: func() (*cfgpkg.File, error) {
			return cfgpkg.LoadFile(configPath)
		},
		Save: cfgpkg.Save,
	}

	listed, err := svc.ConfigList()
	if err != nil {
		t.Fatalf("config list: %v", err)
	}
	if listed.Global.Target == nil || *listed.Global.Target != "~/bin" {
		t.Fatalf("expected listed global target, got %#v", listed.Global.Target)
	}
	if _, ok := listed.Packages["fzf"]; !ok {
		t.Fatal("expected listed package fzf")
	}

	value, err := svc.ConfigGet("global.target")
	if err != nil {
		t.Fatalf("config get global.target: %v", err)
	}
	if value != "~/bin" {
		t.Fatalf("expected global.target to be ~/bin, got %q", value)
	}

	value, err = svc.ConfigGet("packages.fzf.repo")
	if err != nil {
		t.Fatalf("config get packages.fzf.repo: %v", err)
	}
	if value != "junegunn/fzf" {
		t.Fatalf("expected packages.fzf.repo to be junegunn/fzf, got %q", value)
	}

	value, err = svc.ConfigGet("pkg.fzf.repo")
	if err != nil {
		t.Fatalf("config get pkg.fzf.repo: %v", err)
	}
	if value != "junegunn/fzf" {
		t.Fatalf("expected pkg.fzf.repo to be junegunn/fzf, got %q", value)
	}

	if err := svc.ConfigSet("global.cache_dir", "~/.cache/eget"); err != nil {
		t.Fatalf("config set cache_dir: %v", err)
	}

	value, err = svc.ConfigGet("global.cache_dir")
	if err != nil {
		t.Fatalf("config get updated global.cache_dir: %v", err)
	}
	if value != "~/.cache/eget" {
		t.Fatalf("expected updated global.cache_dir, got %q", value)
	}

	if err := svc.ConfigSet("global.proxy_url", "http://127.0.0.1:7890"); err != nil {
		t.Fatalf("config set proxy_url: %v", err)
	}

	value, err = svc.ConfigGet("global.proxy_url")
	if err != nil {
		t.Fatalf("config get updated global.proxy_url: %v", err)
	}
	if value != "http://127.0.0.1:7890" {
		t.Fatalf("expected updated global.proxy_url, got %q", value)
	}

	if err := svc.ConfigSet("global.user_agent", "custom-agent/1.0"); err != nil {
		t.Fatalf("config set user_agent: %v", err)
	}

	value, err = svc.ConfigGet("global.user_agent")
	if err != nil {
		t.Fatalf("config get updated global.user_agent: %v", err)
	}
	if value != "custom-agent/1.0" {
		t.Fatalf("expected updated global.user_agent, got %q", value)
	}

	if err := svc.ConfigSet("global.target", "~/.local/bin"); err != nil {
		t.Fatalf("config set: %v", err)
	}

	value, err = svc.ConfigGet("global.target")
	if err != nil {
		t.Fatalf("config get updated global.target: %v", err)
	}
	if value != "~/.local/bin" {
		t.Fatalf("expected updated global.target, got %q", value)
	}
}

func TestConfigPathInfo(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")
	cacheDir := filepath.Join(tmp, "cache")
	binDir := filepath.Join(tmp, "bin")
	sdkDir := filepath.Join(tmp, "sdks")
	writeConfigFile(t, configPath, `
[global]
target = "`+filepath.ToSlash(binDir)+`"
cache_dir = "`+filepath.ToSlash(cacheDir)+`"
sdk_target = "`+filepath.ToSlash(sdkDir)+`"
`)
	assert.NoErr(t, os.MkdirAll(cacheDir, 0o755))

	svc := ConfigService{
		ConfigPath: configPath,
		Load: func() (*cfgpkg.File, error) {
			return cfgpkg.LoadFile(configPath)
		},
	}

	tests := []struct {
		target string
		path   string
		exists bool
	}{
		{target: "", path: configPath, exists: true},
		{target: "config_file", path: configPath, exists: true},
		{target: "config_dir", path: tmp, exists: true},
		{target: "env_file", path: filepath.Join(tmp, ".env"), exists: false},
		{target: "bin_dir", path: binDir, exists: false},
		{target: "cache_dir", path: cacheDir, exists: true},
		{target: "sdk_dir", path: sdkDir, exists: false},
		{target: "pkg_store_file", path: filepath.Join(tmp, "installed.toml"), exists: false},
		{target: "sdk_store_file", path: filepath.Join(tmp, "sdk.installed.json"), exists: false},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got, err := svc.ConfigPathInfo(tt.target)
			assert.NoErr(t, err)
			assert.Eq(t, filepath.Clean(tt.path), filepath.Clean(got.Path))
			assert.Eq(t, tt.exists, got.Exists)
		})
	}
}

func TestConfigPathInfoRejectsUnknownTarget(t *testing.T) {
	_, err := (ConfigService{}).ConfigPathInfo("unknown")
	assert.Err(t, err)
}

func writeConfigFile(t *testing.T, path, content string) {
	t.Helper()
	if err := cfgpkg.Save(path, mustLoadFromString(t, content)); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func writeRawConfigFile(t *testing.T, path, content string) {
	t.Helper()
	assert.NoErr(t, os.WriteFile(path, []byte(content), 0o644))
}

func mustLoadFromString(t *testing.T, content string) *cfgpkg.File {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "eget.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	cfg, err := cfgpkg.LoadFile(path)
	if err != nil {
		t.Fatalf("load file: %v", err)
	}
	return cfg
}
