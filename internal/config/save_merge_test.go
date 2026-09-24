package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

// The merge rules themselves are covered by gookit/ext/tomlkit. These cases keep
// the eget integration honest: the rendered document comes from gookit/config,
// and the file on disk keeps its comments.

const mergeFixture = `# my eget config
# second header line
[global]
# keep this comment
target = "~/.local/bin" # inline note
user_agent = "eget/1.0"

[packages.fd]
repo = "sharkdp/fd"

[cache_mirror]
enable = false
`

func writeMergeFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "eget.toml")
	assert.NoErr(t, os.WriteFile(path, []byte(mergeFixture), 0o644))
	return path
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	assert.NoErr(t, err)
	return string(data)
}

func TestSaveMergedKeepsCommentsThroughTheConfigEngine(t *testing.T) {
	path := writeMergeFixture(t)

	file, err := LoadFile(path)
	assert.NoErr(t, err)
	assert.NoErr(t, SetByPath(file, "global.target", "/opt/bin"))
	assert.NoErr(t, SaveMerged(path, file))

	got := readFileString(t, path)
	assert.StrContains(t, got, "# my eget config")
	assert.StrContains(t, got, "# second header line")
	assert.StrContains(t, got, "# keep this comment")
	assert.StrContains(t, got, `target = "/opt/bin"`)
	assert.StrContains(t, got, "# inline note")
	// Untouched tables and keys keep their own lines.
	assert.StrContains(t, got, "[packages.fd]\nrepo = \"sharkdp/fd\"")
	assert.StrContains(t, got, "\nuser_agent = \"eget/1.0\"\n")
	// The rendered defaults do not leak in as new tables.
	assert.False(t, strings.Contains(got, "[api_cache]"))
	assert.False(t, strings.Contains(got, "[web]"))
}

func TestSaveMergedRemovesTableAndWritesFreshFile(t *testing.T) {
	path := writeMergeFixture(t)

	file, err := LoadFile(path)
	assert.NoErr(t, err)
	delete(file.Packages, "fd")
	assert.NoErr(t, SaveMerged(path, file))

	got := readFileString(t, path)
	assert.False(t, strings.Contains(got, "[packages.fd]"))
	assert.StrContains(t, got, "# my eget config")

	// A missing file is written in full.
	fresh := filepath.Join(t.TempDir(), "nested", "eget.toml")
	assert.NoErr(t, SaveMerged(fresh, file))
	assert.StrContains(t, readFileString(t, fresh), "[cache_mirror]")
}
