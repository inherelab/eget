package install

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestBinaryChooserSkipsMetadataFiles(t *testing.T) {
	chooser := NewBinaryChooser("bd")

	t.Run("keeps extensionless binary candidate", func(t *testing.T) {
		direct, possible := chooser.Choose("bd", false, 0)
		assert.True(t, direct)
		assert.True(t, possible)
	})

	t.Run("skips extensionless metadata files", func(t *testing.T) {
		for _, name := range []string{"LICENSE", "README", "CHANGELOG", "NOTICE"} {
			direct, possible := chooser.Choose(name, false, 0)
			assert.False(t, direct)
			assert.False(t, possible)
		}
	})
}

func TestFileChooserSupportsExcludePatterns(t *testing.T) {
	chooser, err := NewFileChooser("*.exe,^*x86*,^*.sig")
	assert.NoErr(t, err)

	tests := []struct {
		name string
		want bool
	}{
		{name: "bin/tool-win64.exe", want: true},
		{name: "bin/tool-x86.exe", want: false},
		{name: "bin/tool-win64.exe.sig", want: false},
		{name: "docs/readme.txt", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			direct, possible := chooser.Choose(tt.name, false, 0)
			assert.False(t, direct)
			assert.Eq(t, tt.want, possible)
		})
	}
}

func TestFileChooserMatchesArchivePathsWithSlashGlobs(t *testing.T) {
	tests := []struct {
		expr string
		name string
		want bool
	}{
		{expr: "x64/*", name: `x64\WinDirStat.exe`, want: true},
		{expr: "x64/*.exe", name: `x64\WinDirStat.exe`, want: true},
		{expr: "x64/WinDirStat.exe", name: `x64\WinDirStat.exe`, want: true},
		{expr: "x64/*", name: `x86\WinDirStat.exe`, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.expr+" "+tt.name, func(t *testing.T) {
			chooser, err := NewFileChooser(tt.expr)
			assert.NoErr(t, err)

			direct, possible := chooser.Choose(tt.name, false, 0)
			assert.False(t, direct)
			assert.Eq(t, tt.want, possible)
		})
	}
}

func TestFileChooserExcludeOnlyDefaultsToAllFiles(t *testing.T) {
	chooser, err := NewFileChooser("^*x86*,^*.sig")
	assert.NoErr(t, err)

	tests := []struct {
		name string
		want bool
	}{
		{name: "bin/tool-win64.exe", want: true},
		{name: "bin/tool-x86.exe", want: false},
		{name: "bin/tool-win64.exe.sig", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			direct, possible := chooser.Choose(tt.name, false, 0)
			assert.False(t, direct)
			assert.Eq(t, tt.want, possible)
		})
	}
}

func TestFileChooserSupportsRegexPatterns(t *testing.T) {
	tests := []struct {
		expr string
		name string
		want bool
	}{
		{expr: `REG:(?i)\.(hlf|lng)$`, name: `docs\English.HLF`, want: true},
		{expr: `REG:^plugins/.+\.dll$`, name: `plugins\foo\bar.dll`, want: true},
		{expr: `REG:(?i)\.(hlf|lng)$`, name: `docs\readme.txt`, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.expr+" "+tt.name, func(t *testing.T) {
			chooser, err := NewFileChooser(tt.expr)
			assert.NoErr(t, err)
			_, possible := chooser.Choose(tt.name, false, 0)
			assert.Eq(t, tt.want, possible)
		})
	}
}

func TestFileChooserSupportsRegexExcludePatterns(t *testing.T) {
	chooser, err := NewFileChooser(`*.exe,^REG:(?i)(^|/)x86/`)
	assert.NoErr(t, err)

	_, possible := chooser.Choose(`bin\x64\tool.exe`, false, 0)
	assert.True(t, possible)
	_, possible = chooser.Choose(`bin\x86\tool.exe`, false, 0)
	assert.False(t, possible)
}

func TestFileChooserRegexExcludeOnlyDefaultsToAllFiles(t *testing.T) {
	chooser, err := NewFileChooser(`^REG:(?i)\.(map|cmd|md|txt|diz)$`)
	assert.NoErr(t, err)

	_, possible := chooser.Choose(`bin\Far.exe`, false, 0)
	assert.True(t, possible)
	_, possible = chooser.Choose(`docs\README.md`, false, 0)
	assert.False(t, possible)
}

func TestFileChooserRejectsInvalidRegex(t *testing.T) {
	for _, expr := range []string{`REG:`, `^REG:`, `REG:[`} {
		t.Run(expr, func(t *testing.T) {
			_, err := NewFileChooser(expr)
			assert.Err(t, err)
		})
	}
}
