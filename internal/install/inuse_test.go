package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestOpenReplaceFilePlainOverwrite(t *testing.T) {
	t.Run("fresh create leaves no backup", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "tool.exe")

		out, err := openReplaceFile(target, 0o666)
		assert.Nil(t, err)
		assert.Nil(t, out.Close())
		_, err = os.Stat(target)
		assert.Nil(t, err)
		_, err = os.Stat(target + ".old")
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("overwrite existing removes stale backup", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "tool.exe")
		assert.Nil(t, os.WriteFile(target, []byte("old-content"), 0o666))
		assert.Nil(t, os.WriteFile(target+".old", []byte("stale"), 0o666))

		out, err := openReplaceFile(target, 0o666)
		assert.Nil(t, err)
		_, err = out.Write([]byte("new-content"))
		assert.Nil(t, err)
		assert.Nil(t, out.Close())

		got, err := os.ReadFile(target)
		assert.Nil(t, err)
		assert.Eq(t, "new-content", string(got))
		_, err = os.Stat(target + ".old")
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("missing parent dir keeps original error", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "missing", "tool.exe")

		_, err := openReplaceFile(target, 0o666)
		assert.NotNil(t, err)
		assert.True(t, os.IsNotExist(err))
	})
}
