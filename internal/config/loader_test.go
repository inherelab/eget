package config

import (
	"path/filepath"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestNewFileUsesDefaultSDKExtMap(t *testing.T) {
	cfg := NewFile()

	assert.Eq(t, "zip", cfg.Global.SDKExtMap["windows"])
	assert.Eq(t, "tar.gz", cfg.Global.SDKExtMap["linux"])
	assert.Eq(t, "tar.gz", cfg.Global.SDKExtMap["darwin"])
}

func TestNewFileEnablesAPICacheByDefault(t *testing.T) {
	cfg := NewFile()

	assert.True(t, cfg.ApiCache.Enable != nil && *cfg.ApiCache.Enable)
}

func TestLoadFileKeepsDefaultSDKExtMapWhenGlobalOmitsIt(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")
	writeTestFile(t, configPath, `
[global]
target = "~/bin"
`)

	cfg, err := LoadFile(configPath)
	assert.NoErr(t, err)
	assert.Eq(t, "zip", cfg.Global.SDKExtMap["windows"])
	assert.Eq(t, "tar.gz", cfg.Global.SDKExtMap["linux"])
	assert.Eq(t, "tar.gz", cfg.Global.SDKExtMap["darwin"])
}
