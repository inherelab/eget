package sourceforge

import (
	"testing"

	"github.com/gookit/goutil/testutil/assert"
)

func TestVersionFromText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "plain version", text: "2.16.44", want: "2.16.44"},
		{name: "setup file", text: "WinMerge-2.16.44-x64-Setup.exe", want: "2.16.44"},
		{name: "source archive", text: "winmerge-2.16.44-src.zip", want: "2.16.44"},
		{name: "no version", text: "stable", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Eq(t, tt.want, VersionFromText(tt.text))
		})
	}
}

func TestLatestVersionFile(t *testing.T) {
	files := []File{
		{Name: "2.16.42", FullPath: "/stable/2.16.42", Type: TypeDirectory},
		{Name: "readme.txt", FullPath: "/stable/readme.txt", Type: TypeFile},
		{Name: "2.16.44", FullPath: "/stable/2.16.44", Type: TypeDirectory},
	}

	file, ok := LatestVersionFile(files)

	assert.True(t, ok)
	assert.Eq(t, "2.16.44", file.Name)
}
