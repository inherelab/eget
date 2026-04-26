package config

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathGetAndSet(t *testing.T) {
	cfg := NewFile()

	if err := SetByPath(cfg, "global.target", "~/.local/bin"); err != nil {
		t.Fatalf("set global.target: %v", err)
	}
	if err := SetByPath(cfg, "api_cache.enable", "true"); err != nil {
		t.Fatalf("set api_cache.enable: %v", err)
	}
	if err := SetByPath(cfg, "api_cache.cache_time", "300"); err != nil {
		t.Fatalf("set api_cache.cache_time: %v", err)
	}
	if err := SetByPath(cfg, "packages.fzf.repo", "junegunn/fzf"); err != nil {
		t.Fatalf("set packages.fzf.repo: %v", err)
	}
	if err := SetByPath(cfg, "packages.fzf.asset_filters", "linux,amd64"); err != nil {
		t.Fatalf("set packages.fzf.asset_filters: %v", err)
	}
	if err := SetByPath(cfg, "packages.fzf.extract_all", "true"); err != nil {
		t.Fatalf("set packages.fzf.extract_all: %v", err)
	}

	target, ok := GetByPath(cfg, "global.target")
	if !ok || target != "~/.local/bin" {
		t.Fatalf("expected global.target to be set, got %#v ok=%t", target, ok)
	}
	cacheTime, ok := GetByPath(cfg, "api_cache.cache_time")
	if !ok || cacheTime != 300 {
		t.Fatalf("expected api_cache.cache_time to be 300, got %#v ok=%t", cacheTime, ok)
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
		Target:     &repoTarget,
		System:     &repoSystem,
		ExtractAll: boolPtr(true),
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

func TestSaveAndLoadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	cfg := NewFile()
	target := "~/.local/bin"
	quiet := true
	cacheTime := 300
	repo := "junegunn/fzf"
	cfg.Global.Target = &target
	cfg.Global.Quiet = &quiet
	cfg.ApiCache.CacheTime = &cacheTime
	cfg.Packages["fzf"] = Section{Repo: &repo}

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
	if loaded.Global.Quiet == nil || !*loaded.Global.Quiet {
		t.Fatalf("expected round-trip global.quiet, got %#v", loaded.Global.Quiet)
	}
	if loaded.ApiCache.CacheTime == nil || *loaded.ApiCache.CacheTime != 300 {
		t.Fatalf("expected round-trip api_cache.cache_time, got %#v", loaded.ApiCache.CacheTime)
	}
	if loaded.Packages["fzf"].Repo == nil || *loaded.Packages["fzf"].Repo != "junegunn/fzf" {
		t.Fatalf("expected round-trip packages.fzf.repo, got %#v", loaded.Packages["fzf"].Repo)
	}
}
