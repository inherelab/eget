package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gookit/goutil/x/assert"
	"github.com/inherelab/eget/internal/util"
)

func TestLoadWebSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "eget.toml")
	body := `[web]
host = "0.0.0.0"
token = "s3cret"
allow_host = ["*"]
read_only = true
auto_open = true
cache_root = "sdk"
no_cache_index = true

[global]
target = "/tmp/bin"
`
	assert.NoErr(t, os.WriteFile(path, []byte(body), 0o644))

	cfg, err := LoadFile(path)
	assert.NoErr(t, err)

	assert.Eq(t, "0.0.0.0", util.DerefString(cfg.Web.Host))
	assert.Eq(t, "s3cret", util.DerefString(cfg.Web.Token))
	assert.Eq(t, []string{"*"}, cfg.Web.AllowHosts)
	assert.Eq(t, "sdk", util.DerefString(cfg.Web.CacheRoot))
	assert.True(t, cfg.Web.ReadOnly != nil && *cfg.Web.ReadOnly)
	assert.True(t, cfg.Web.AutoOpen != nil && *cfg.Web.AutoOpen)
	assert.True(t, cfg.Web.NoCacheIndex != nil && *cfg.Web.NoCacheIndex)
	assert.True(t, cfg.Web.AllowMutations == nil)

	// web must be a reserved section, never a repo entry.
	if _, ok := cfg.Repos["web"]; ok {
		t.Fatal("web must not be treated as a repo section")
	}
}

func TestSetWebKeysByPath(t *testing.T) {
	cfg := NewFile()

	assert.NoErr(t, SetByPath(cfg, "web.host", "127.0.0.1"))
	assert.NoErr(t, SetByPath(cfg, "web.token", "s3cret"))
	assert.NoErr(t, SetByPath(cfg, "web.allow_host", "192.168.1.10, *.lan"))
	assert.NoErr(t, SetByPath(cfg, "web.read_only", "true"))

	assert.Eq(t, "127.0.0.1", util.DerefString(cfg.Web.Host))
	assert.Eq(t, "s3cret", util.DerefString(cfg.Web.Token))
	assert.Eq(t, []string{"192.168.1.10", "*.lan"}, cfg.Web.AllowHosts)
	assert.True(t, cfg.Web.ReadOnly != nil && *cfg.Web.ReadOnly)

	encoded := encodeConfigFile(cfg).Data()
	section, ok := encoded["web"].(map[string]any)
	if !ok {
		t.Fatalf("encoded config has no web section: %#v", encoded["web"])
	}
	assert.Eq(t, "127.0.0.1", section["host"])
	assert.Eq(t, "s3cret", section["token"])
	assert.Eq(t, []string{"192.168.1.10", "*.lan"}, section["allow_host"])
	assert.Eq(t, true, section["read_only"])
}
