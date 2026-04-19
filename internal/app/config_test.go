package app

import (
	"os"
	"path/filepath"
	"testing"

	cfgpkg "github.com/inherelab/eget/internal/config"
)

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
	if cfg.Global.System == nil || *cfg.Global.System != "" {
		t.Fatalf("expected empty global.system, got %#v", cfg.Global.System)
	}
	if cfg.Global.CacheDir == nil || *cfg.Global.CacheDir != "~/.cache/eget" {
		t.Fatalf("expected default global.cache_dir, got %#v", cfg.Global.CacheDir)
	}
	if cfg.Global.ProxyURL == nil || *cfg.Global.ProxyURL != "" {
		t.Fatalf("expected default global.proxy_url, got %#v", cfg.Global.ProxyURL)
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

func writeConfigFile(t *testing.T, path, content string) {
	t.Helper()
	if err := cfgpkg.Save(path, mustLoadFromString(t, content)); err != nil {
		t.Fatalf("write config: %v", err)
	}
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
