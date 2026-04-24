package configutil

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/gookit/goutil/testutil/assert"
)

func TestManagerSaveLoadAndBind(t *testing.T) {
	t.Run("save load and bind by path", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "eget.toml")

		mgr := NewManager("test-manager")
		err := mgr.Set("global.target", "~/.local/bin", true)
		assert.Nil(t, err)
		err = mgr.Set("packages.fzf.repo", "junegunn/fzf", true)
		assert.Nil(t, err)

		err = mgr.SaveTo(path)
		assert.Nil(t, err)

		loaded, err := LoadManager("test-manager", path)
		assert.Nil(t, err)

		val, ok := loaded.GetValue("global.target", true)
		assert.True(t, ok)
		assert.Eq(t, "~/.local/bin", val)

		var pkg struct {
			Repo string `mapstructure:"repo"`
		}
		err = loaded.BindStruct("packages.fzf", &pkg)
		assert.Nil(t, err)
		assert.Eq(t, "junegunn/fzf", pkg.Repo)
	})
}

func TestManagerDecodeAndDump(t *testing.T) {
	t.Run("decode data and dump toml", func(t *testing.T) {
		mgr := NewManager("test-manager")
		mgr.SetData(map[string]any{
			"global": map[string]any{
				"target": "~/.local/bin",
			},
			"api_cache": map[string]any{
				"enable": true,
			},
		})

		var decoded struct {
			Global struct {
				Target string `mapstructure:"target"`
			} `mapstructure:"global"`
			APICache struct {
				Enable bool `mapstructure:"enable"`
			} `mapstructure:"api_cache"`
		}
		err := mgr.Decode(&decoded)
		assert.Nil(t, err)
		assert.Eq(t, "~/.local/bin", decoded.Global.Target)
		assert.True(t, decoded.APICache.Enable)

		var buf bytes.Buffer
		_, err = mgr.DumpTo(&buf)
		assert.Nil(t, err)
		assert.Contains(t, buf.String(), "target = \"~/.local/bin\"")
	})
}
