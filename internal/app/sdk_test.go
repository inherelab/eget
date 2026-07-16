package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/sdk"
	"github.com/inherelab/eget/internal/util"
)

func TestNewDefaultSDKServiceUsesConfigPathsAndNetworkOptions(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")
	t.Setenv("EGET_CONFIG", configPath)

	cacheDir := filepath.Join(tmp, "cache")
	proxyURL := "http://127.0.0.1:7890"
	userAgent := "custom-agent/1.0"
	apiCacheEnable := true
	apiCacheTime := 180
	disableSSL := true
	chunkConcurrency := 3
	ghproxyHost := "https://gh.example.com"
	cacheMirrorEnable := true
	cacheMirrorURL := "http://mirror.local:8686/"
	cacheMirrorTimeout := 4
	cacheMirrorFallback := false
	cfg := cfgpkg.NewFile()
	cfg.Global.CacheDir = &cacheDir
	cfg.Global.ProxyURL = &proxyURL
	cfg.Global.UserAgent = &userAgent
	cfg.Global.DisableSSL = &disableSSL
	cfg.Global.ChunkConcurrency = &chunkConcurrency
	cfg.ApiCache.Enable = &apiCacheEnable
	cfg.ApiCache.CacheTime = &apiCacheTime
	cfg.Ghproxy.HostURL = &ghproxyHost
	cfg.Ghproxy.Fallbacks = []string{"https://gh2.example.com"}
	cfg.CacheMirror.Enable = &cacheMirrorEnable
	cfg.CacheMirror.URL = &cacheMirrorURL
	cfg.CacheMirror.Timeout = &cacheMirrorTimeout
	cfg.CacheMirror.Fallback = &cacheMirrorFallback

	service, err := NewDefaultSDKService(cfg)
	assert.NoErr(t, err)

	wantStorePath, err := sdk.DefaultStorePath()
	assert.NoErr(t, err)
	assert.Eq(t, cfg, service.Config)
	assert.Eq(t, filepath.Join(cacheDir, "sdk-index"), service.IndexCache.Dir)
	assert.Eq(t, wantStorePath, service.Store.Path)
	assert.Eq(t, proxyURL, service.ClientOpts.ProxyURL)
	assert.Eq(t, userAgent, service.ClientOpts.UserAgent)
	assert.True(t, service.ClientOpts.APICacheEnabled)
	assert.Eq(t, filepath.Join(cacheDir, "api-cache"), service.ClientOpts.APICacheDir)
	assert.Eq(t, apiCacheTime, service.ClientOpts.APICacheTime)
	assert.True(t, service.ClientOpts.DisableSSL)
	assert.Eq(t, chunkConcurrency, service.ClientOpts.ChunkConcurrency)
	assert.False(t, service.ClientOpts.GhproxyEnabled)
	assert.Eq(t, ghproxyHost, service.ClientOpts.GhproxyHostURL)
	assert.Eq(t, []string{"https://gh2.example.com"}, service.ClientOpts.GhproxyFallbacks)
	assert.True(t, service.ClientOpts.CacheMirror.Enable)
	assert.Eq(t, "http://mirror.local:8686", service.ClientOpts.CacheMirror.URL)
	assert.Eq(t, 4*time.Second, service.ClientOpts.CacheMirror.Timeout)
	assert.False(t, service.ClientOpts.CacheMirror.Fallback)
	assert.True(t, service.CacheMirror.Enable)
	assert.Eq(t, "http://mirror.local:8686", service.CacheMirror.URL)
	assert.Eq(t, 4*time.Second, service.CacheMirror.Timeout)
	assert.False(t, service.CacheMirror.Fallback)
}

func TestSDKClientOptionsEnablesAPICachePathForMirrorOnly(t *testing.T) {
	cacheDir := t.TempDir()
	apiCacheEnabled := false
	mirrorEnabled := true
	mirrorURL := "http://mirror.local:8686"
	cfg := cfgpkg.NewFile()
	cfg.Global.CacheDir = &cacheDir
	cfg.ApiCache.Enable = &apiCacheEnabled
	cfg.CacheMirror.Enable = &mirrorEnabled
	cfg.CacheMirror.URL = &mirrorURL

	opts := sdkClientOptionsFromConfig(cfg)
	assert.False(t, opts.APICacheEnabled)
	assert.Eq(t, filepath.Join(cacheDir, "api-cache"), opts.APICacheDir)
	assert.True(t, opts.CacheMirror.Active())
}

func TestNewDefaultSDKServiceSkipsProxyURLWhenNoProxyEnabled(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	proxyURL := "http://127.0.0.1:7890"
	cfg := cfgpkg.NewFile()
	cfg.Global.ProxyURL = &proxyURL

	service, err := NewDefaultSDKService(cfg, true)

	assert.NoErr(t, err)
	assert.Eq(t, "", service.ClientOpts.ProxyURL)
}

func TestNewDefaultSDKServiceUsesHTTPProxyConfig(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	cfg := cfgpkg.NewFile()
	proxyURL := "http://127.0.0.1:10801"
	cfg.HTTPProxy.URL = &proxyURL
	cfg.HTTPProxy.Exclude = []string{"mydev.com"}

	service, err := NewDefaultSDKService(cfg)

	assert.NoErr(t, err)
	assert.Eq(t, proxyURL, service.ClientOpts.ProxyURL)
	assert.Eq(t, []string{"mydev.com"}, service.ClientOpts.ProxyExclude)
}

func TestNewDefaultSDKServiceHTTPProxyEnableFalseDisables(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	cfg := cfgpkg.NewFile()
	proxyURL := "http://127.0.0.1:10801"
	enabled := false
	cfg.HTTPProxy.URL = &proxyURL
	cfg.HTTPProxy.Enable = &enabled

	service, err := NewDefaultSDKService(cfg)

	assert.NoErr(t, err)
	assert.Eq(t, "", service.ClientOpts.ProxyURL)
	assert.Eq(t, []string(nil), service.ClientOpts.ProxyExclude)
}

func TestNewDefaultSDKServiceHTTPProxyPrefersNewBlockOverLegacy(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	cfg := cfgpkg.NewFile()
	legacyURL := "http://127.0.0.1:10801"
	currentURL := "http://127.0.0.1:10802"
	cfg.Global.ProxyURL = &legacyURL
	cfg.HTTPProxy.URL = &currentURL

	service, err := NewDefaultSDKService(cfg)

	assert.NoErr(t, err)
	assert.Eq(t, currentURL, service.ClientOpts.ProxyURL)
}

func TestNewDefaultSDKServiceNoProxyHostListBecomesProxyExclude(t *testing.T) {
	t.Setenv("NO_PROXY", "mydev.com,*.corp.local")
	cfg := cfgpkg.NewFile()
	proxyURL := "http://127.0.0.1:10801"
	cfg.HTTPProxy.URL = &proxyURL

	service, err := NewDefaultSDKService(cfg)

	assert.NoErr(t, err)
	assert.Eq(t, proxyURL, service.ClientOpts.ProxyURL)
	assert.Eq(t, []string{"mydev.com", "*.corp.local"}, service.ClientOpts.ProxyExclude)
}

func TestNewDefaultSDKServiceNoProxyDisableValueDisablesProxy(t *testing.T) {
	t.Setenv("NO_PROXY", "1")
	cfg := cfgpkg.NewFile()
	proxyURL := "http://127.0.0.1:10801"
	cfg.HTTPProxy.URL = &proxyURL

	service, err := NewDefaultSDKService(cfg)

	assert.NoErr(t, err)
	assert.Eq(t, "", service.ClientOpts.ProxyURL)
	assert.Eq(t, []string(nil), service.ClientOpts.ProxyExclude)
}

func TestNewDefaultSDKServiceLegacyProxyFallbackWhenHTTPProxyAbsent(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	cfg := cfgpkg.NewFile()
	proxyURL := "http://127.0.0.1:10801"
	cfg.Global.ProxyURL = &proxyURL

	service, err := NewDefaultSDKService(cfg)

	assert.NoErr(t, err)
	assert.Eq(t, proxyURL, service.ClientOpts.ProxyURL)
	assert.Eq(t, []string(nil), service.ClientOpts.ProxyExclude)
}

func TestNewDefaultSDKServiceLoadsConfigWhenNil(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")
	t.Setenv("EGET_CONFIG", configPath)

	cacheDir := filepath.Join(tmp, "cache")
	cfg := cfgpkg.NewFile()
	cfg.Global.CacheDir = util.StringPtr(cacheDir)
	assert.NoErr(t, cfgpkg.Save(configPath, cfg))

	service, err := NewDefaultSDKService(nil)
	assert.NoErr(t, err)

	assert.Eq(t, filepath.Join(cacheDir, "sdk-index"), service.IndexCache.Dir)
}
