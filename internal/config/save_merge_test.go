package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

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

func TestSaveMergedKeepsCommentsAndUntouchedTables(t *testing.T) {
	path := writeMergeFixture(t)

	file, err := LoadFile(path)
	assert.NoErr(t, err)
	assert.NoErr(t, SetByPath(file, "global.target", "/opt/bin"))
	assert.NoErr(t, SaveMerged(path, file))

	got := readFileString(t, path)
	// Comments and layout survive, both at the top of the file and inside the
	// table that changed.
	assert.StrContains(t, got, "# my eget config")
	assert.StrContains(t, got, "# second header line")
	assert.StrContains(t, got, "# keep this comment")
	assert.StrContains(t, got, `target = "/opt/bin"`)
	// Untouched tables are kept byte for byte.
	assert.StrContains(t, got, "repo = \"sharkdp/fd\"")
	assert.StrContains(t, got, "[cache_mirror]")
	assert.StrContains(t, got, "enable = false")
	// The rendered defaults do not leak in as brand new tables.
	assert.False(t, contains(got, "[api_cache]"))
	assert.False(t, contains(got, "[web]"))
	assert.False(t, contains(got, "[sdk]"))
	// A header-only table (its content lives in [packages.fd]) is not added.
	assert.False(t, contains(got, "\n[packages]\n"))
	// An untouched key inside a changed table keeps its original line: no
	// renderer indentation, inline comment intact.
	assert.StrContains(t, got, "\nuser_agent = \"eget/1.0\"\n")
	// The changed key keeps its own comments, with the new value.
	assert.StrContains(t, got, "# keep this comment")
	assert.StrContains(t, got, "# inline note")
}

func TestSaveMergedAppliesNewAndRemovedTables(t *testing.T) {
	path := writeMergeFixture(t)

	file, err := LoadFile(path)
	assert.NoErr(t, err)
	// Remove one package and add another.
	delete(file.Packages, "fd")
	section := Section{Repo: strPtr("sharkdp/bat")}
	file.Packages["bat"] = section
	assert.NoErr(t, SaveMerged(path, file))

	got := readFileString(t, path)
	assert.False(t, contains(got, "[packages.fd]"))
	assert.StrContains(t, got, "[packages.bat]")
	assert.StrContains(t, got, "sharkdp/bat")
	// The documents's own comments are still there.
	assert.StrContains(t, got, "# my eget config")
	assert.StrContains(t, got, "# keep this comment")
}

func TestSaveMergedWritesFreshFileAndFallsBackOnBrokenInput(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nested", "eget.toml")
	file := NewFile()
	section := Section{Repo: strPtr("sharkdp/fd")}
	file.Packages["fd"] = section

	assert.NoErr(t, SaveMerged(missing, file))
	assert.StrContains(t, readFileString(t, missing), "[packages]")

	broken := filepath.Join(dir, "broken.toml")
	assert.NoErr(t, os.WriteFile(broken, []byte("this is not = = toml\n"), 0o644))
	assert.NoErr(t, SaveMerged(broken, file))
	// A broken document is replaced wholesale rather than merged.
	assert.StrContains(t, readFileString(t, broken), "[packages]")
}

func strPtr(value string) *string { return &value }

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(needle) == 0 ||
		indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
