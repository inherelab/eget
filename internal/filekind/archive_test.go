package filekind

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

// TestKindExact walks the ExtSetExtractor suffix list: each suffix maps to its
// own kind, multi-segment suffixes win over their single-segment tails, and the
// case-sensitive axis leaves upper-case names unknown.
func TestKindExact(t *testing.T) {
	cases := []struct {
		name string
		want Kind
	}{
		{"tool_1.0_linux_amd64.tar.gz", KindTarGz},
		{"tool_1.0_linux_amd64.tgz", KindTarGz},
		{"tool.tar.bz2", KindTarBz2},
		{"tool.tbz", KindTarBz2},
		{"tool.tar.xz", KindTarXz},
		{"tool.txz", KindTarXz},
		{"tool.tar.zst", KindTarZst},
		{"tool.tar", KindTar},
		{"tool.zip", KindZip},
		{"tool.7z", KindSevenZip},
		{"tool.gz", KindGz},
		{"tool.bz2", KindBz2},
		{"tool.xz", KindXz},
		{"tool.zst", KindZst},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Eq(t, tc.want, KindExact(tc.name))
		})
	}

	t.Run("multi segment beats single segment", func(t *testing.T) {
		assert.Eq(t, KindTarGz, KindExact("x.tar.gz"))
		assert.NotEq(t, KindGz, KindExact("x.tar.gz"))
	})

	t.Run("unknown and upper case", func(t *testing.T) {
		assert.Eq(t, KindUnknown, KindExact("tool.exe"))
		assert.Eq(t, KindUnknown, KindExact("TOOL.TAR.GZ"))
		// .tar.br is not part of the extractor set — it stays a plain file.
		assert.Eq(t, KindUnknown, KindExact("tool.tar.br"))
	})
}

func TestMultiExtFold(t *testing.T) {
	t.Run("matches and preserves the original case", func(t *testing.T) {
		assert.Eq(t, ".tar.gz", MultiExtFold("tool_1.0_linux_x64.tar.gz", ExtSetArchiveMulti))
		assert.Eq(t, ".tar.br", MultiExtFold("tool_1.0_linux_x64.tar.br", ExtSetArchiveMulti))
		assert.Eq(t, ".tar.lz4", MultiExtFold("tool_1.0_linux_x64.tar.lz4", ExtSetArchiveMulti))
		assert.Eq(t, ".TAR.BR", MultiExtFold("TOOL-x64.TAR.BR", ExtSetArchiveMulti))
	})

	t.Run("falls back to path.Ext", func(t *testing.T) {
		assert.Eq(t, ".zip", MultiExtFold("tool.zip", ExtSetArchiveMulti))
		assert.Eq(t, "", MultiExtFold("tool", ExtSetArchiveMulti))
		// .tar.zst is part of the set, so the multi-segment form is reported.
		assert.Eq(t, ".tar.zst", MultiExtFold("tool.tar.zst", ExtSetArchiveMulti))
	})
}

func TestMultiExtExact(t *testing.T) {
	t.Run("returns the set literal on a match", func(t *testing.T) {
		ext, ok := MultiExtExact("tool_1.0_linux_x64.tar.gz", ExtSetArchiveMulti)
		assert.True(t, ok)
		assert.Eq(t, ".tar.gz", ext)
		assert.Eq(t, "tar.gz", ext[1:])
	})

	t.Run("matches the multi-segment forms", func(t *testing.T) {
		for _, name := range []string{"tool.tar.zst", "tool.tar.br", "tool.tar.lz4"} {
			ext, ok := MultiExtExact(name, ExtSetArchiveMulti)
			assert.True(t, ok, name)
			assert.NotEq(t, "", ext, name)
		}
	})

	t.Run("case sensitive axis", func(t *testing.T) {
		ext, ok := MultiExtExact("TOOL.TAR.GZ", ExtSetArchiveMulti)
		assert.False(t, ok)
		assert.Eq(t, ".GZ", ext)
	})
}

func TestIsArchiveFold(t *testing.T) {
	t.Run("extract all set", func(t *testing.T) {
		for _, name := range []string{"tool.zip", "tool.7z", "tool.tar", "tool.tar.gz", "tool.tgz",
			"tool.tar.bz2", "tool.tbz", "tool.tar.xz", "tool.txz", "tool.tar.zst"} {
			assert.True(t, IsArchiveFold(name, ExtSetExtractAll), name)
		}
		assert.True(t, IsArchiveFold("TOOL.TAR.ZST", ExtSetExtractAll))
		// Not in the extract-all set: single-file compressors and .tar.br.
		assert.False(t, IsArchiveFold("tool.gz", ExtSetExtractAll))
		assert.False(t, IsArchiveFold("tool.tar.br", ExtSetExtractAll))
	})
}

func TestIsDownloadFold(t *testing.T) {
	t.Run("known download extensions", func(t *testing.T) {
		for _, name := range []string{"a.zip", "a.gz", "a.tgz", "a.xz", "a.bz2", "a.zst", "a.7z", "a.rar"} {
			assert.True(t, IsDownloadFold(name), name)
		}
		assert.True(t, IsDownloadFold(`C:\cache\pkg-cache\A.ZIP`))
		// path.Ext semantics: a.tar.gz reports .gz, which is in the set.
		assert.True(t, IsDownloadFold("a.tar.gz"))
	})

	t.Run("unknown extensions", func(t *testing.T) {
		assert.False(t, IsDownloadFold("a.tar.br"))
		assert.False(t, IsDownloadFold("a.exe"))
		assert.False(t, IsDownloadFold("a"))
		assert.False(t, IsDownloadFold(`C:\dir.v2\file`))
	})
}
