package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveConfigPathPrefersEnv(t *testing.T) {
	tmp := t.TempDir()
	envPath := filepath.Join(tmp, "env.toml")
	homePath := filepath.Join(tmp, "home")

	writeTestFile(t, envPath, "title = 'env'\n")

	path, err := resolveConfigPath(pathOptions{
		HomeDir: homePath,
		GOOS:    runtime.GOOS,
		LookupEnv: func(key string) (string, bool) {
			if key == "EGET_CONFIG" {
				return envPath, true
			}
			return "", false
		},
	})
	if err != nil {
		t.Fatalf("resolve path: %v", err)
	}

	if path != envPath {
		t.Fatalf("expected env path %q, got %q", envPath, path)
	}
}

func TestResolveConfigPathFallsBackToDotfile(t *testing.T) {
	tmp := t.TempDir()
	homePath := filepath.Join(tmp, "home")
	dotfile := filepath.Join(homePath, ".eget.toml")

	writeTestFile(t, dotfile, "title = 'home'\n")

	path, err := resolveConfigPath(pathOptions{
		HomeDir:   homePath,
		GOOS:      runtime.GOOS,
		LookupEnv: func(string) (string, bool) { return "", false },
	})
	if err != nil {
		t.Fatalf("resolve path: %v", err)
	}

	if path != dotfile {
		t.Fatalf("expected dotfile path %q, got %q", dotfile, path)
	}
}

func TestResolveConfigPathFallsBackToOSConfigDir(t *testing.T) {
	tmp := t.TempDir()
	homePath := filepath.Join(tmp, "home")

	testCases := []struct {
		name     string
		goos     string
		envKey   string
		envValue string
		wantPath string
	}{
		{
			name:     "xdg",
			goos:     "linux",
			envKey:   "XDG_CONFIG_HOME",
			envValue: filepath.Join(tmp, "xdg"),
			wantPath: filepath.Join(tmp, "xdg", "eget", "eget.toml"),
		},
		{
			name:     "windows uses xdg env when set",
			goos:     "windows",
			envKey:   "XDG_CONFIG_HOME",
			envValue: filepath.Join(tmp, "xdg-win"),
			wantPath: filepath.Join(tmp, "xdg-win", "eget", "eget.toml"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			writeTestFile(t, tc.wantPath, "title = 'os'\n")

			path, err := resolveConfigPath(pathOptions{
				HomeDir: homePath,
				GOOS:    tc.goos,
				LookupEnv: func(key string) (string, bool) {
					if key == tc.envKey {
						return tc.envValue, true
					}
					return "", false
				},
			})
			if err != nil {
				t.Fatalf("resolve path: %v", err)
			}

			if path != tc.wantPath {
				t.Fatalf("expected os config path %q, got %q", tc.wantPath, path)
			}
		})
	}
}

func TestResolveConfigPathSkipsDotfileWhenEnvPathMissing(t *testing.T) {
	tmp := t.TempDir()
	homePath := filepath.Join(tmp, "home")
	dotfile := filepath.Join(homePath, ".eget.toml")
	fallbackPath := filepath.Join(tmp, "xdg", "eget", "eget.toml")

	writeTestFile(t, dotfile, "title = 'home'\n")
	writeTestFile(t, fallbackPath, "title = 'fallback'\n")

	path, err := resolveConfigPath(pathOptions{
		HomeDir: homePath,
		GOOS:    "linux",
		LookupEnv: func(key string) (string, bool) {
			switch key {
			case "EGET_CONFIG":
				return filepath.Join(tmp, "missing.toml"), true
			case "XDG_CONFIG_HOME":
				return filepath.Join(tmp, "xdg"), true
			default:
				return "", false
			}
		},
	})
	if err != nil {
		t.Fatalf("resolve path: %v", err)
	}

	if path != fallbackPath {
		t.Fatalf("expected fallback path %q when env config is missing, got %q", fallbackPath, path)
	}
}

func TestResolveWritablePathDefaultsToOSConfigDir(t *testing.T) {
	tmp := t.TempDir()
	homePath := filepath.Join(tmp, "home")
	xdgHome := filepath.Join(tmp, "xdg")
	wantPath := filepath.Join(xdgHome, "eget", "eget.toml")

	path, err := resolveWritablePath(pathOptions{
		HomeDir: homePath,
		GOOS:    "linux",
		LookupEnv: func(key string) (string, bool) {
			if key == "XDG_CONFIG_HOME" {
				return xdgHome, true
			}
			return "", false
		},
	})
	if err != nil {
		t.Fatalf("resolve writable path: %v", err)
	}

	if path != wantPath {
		t.Fatalf("expected writable path %q, got %q", wantPath, path)
	}
}

func TestResolveWritablePathDefaultsToHomeConfigDirOnWindows(t *testing.T) {
	tmp := t.TempDir()
	homePath := filepath.Join(tmp, "home")
	wantPath := filepath.Join(homePath, ".config", "eget", "eget.toml")

	path, err := resolveWritablePath(pathOptions{
		HomeDir:   homePath,
		GOOS:      "windows",
		LookupEnv: func(string) (string, bool) { return "", false },
	})
	if err != nil {
		t.Fatalf("resolve writable path: %v", err)
	}

	if path != wantPath {
		t.Fatalf("expected writable path %q, got %q", wantPath, path)
	}
}

func TestLoadFileSupportsLegacyRepoSections(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	writeTestFile(t, configPath, `
[global]
target = "~/bin"
quiet = true
github_token = "token"

["owner/repo"]
asset_filters = ["linux", "!arm"]
download_only = true
`)

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("load file: %v", err)
	}

	if cfg.Global.Target == nil || *cfg.Global.Target != "~/bin" {
		t.Fatalf("expected global target to be loaded, got %#v", cfg.Global.Target)
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
enable = true
host_url = "https://gh.felicity.ac.cn"
support_api = true
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
	if cfg.Ghproxy.Enable == nil || !*cfg.Ghproxy.Enable {
		t.Fatalf("expected ghproxy.enable=true, got %#v", cfg.Ghproxy.Enable)
	}
	if cfg.Ghproxy.HostURL == nil || *cfg.Ghproxy.HostURL != "https://gh.felicity.ac.cn" {
		t.Fatalf("expected ghproxy.host_url, got %#v", cfg.Ghproxy.HostURL)
	}
	if cfg.Ghproxy.SupportAPI == nil || !*cfg.Ghproxy.SupportAPI {
		t.Fatalf("expected ghproxy.support_api=true, got %#v", cfg.Ghproxy.SupportAPI)
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

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
