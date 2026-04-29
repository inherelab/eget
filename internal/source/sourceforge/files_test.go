package sourceforge

import (
	"strings"
	"testing"

	"github.com/gookit/goutil/testutil/assert"
)

func TestParseFilesPage(t *testing.T) {
	body := []byte(`
<html>
<body>
<script>
net.sf.files = {
  "2.16.44": {
    "name": "2.16.44",
    "path": "/winmerge/stable/2.16.44",
    "url": "https://sourceforge.net/projects/winmerge/files/stable/2.16.44/",
    "full_path": "/stable/2.16.44",
    "type": "d",
    "downloadable": false
  },
  "WinMerge-2.16.44-x64-Setup.exe": {
    "name": "WinMerge-2.16.44-x64-Setup.exe",
    "path": "/winmerge/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe",
    "download_url": "https://downloads.sourceforge.net/project/winmerge/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe",
    "url": "https://sourceforge.net/projects/winmerge/files/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe/download",
    "full_path": "/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe",
    "type": "f",
    "downloadable": true
  }
};
net.sf.staging_days = 3;
</script>
</body>
</html>`)

	files, err := ParseFilesPage(body)
	if err != nil {
		t.Fatalf("ParseFilesPage(): %v", err)
	}

	assert.Len(t, files, 2)
	assert.Eq(t, "2.16.44", files[0].Name)
	assert.Eq(t, TypeDirectory, files[0].Type)
	assert.False(t, files[0].Downloadable)
	assert.Eq(t, "WinMerge-2.16.44-x64-Setup.exe", files[1].Name)
	assert.Eq(t, TypeFile, files[1].Type)
	assert.True(t, files[1].Downloadable)
	assert.Eq(t, "https://downloads.sourceforge.net/project/winmerge/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe", files[1].DownloadURL)
}

func TestParseFilesPageRejectsMissingData(t *testing.T) {
	_, err := ParseFilesPage([]byte(""))
	if err == nil || !strings.Contains(err.Error(), "sourceforge files data not found") {
		t.Fatalf("expected missing data error, got %v", err)
	}
}

func TestParseFilesPageHandlesJSONStringsWithBracesAndEscapes(t *testing.T) {
	body := []byte(`
<script>
net.sf.files = {
  "escaped": {
    "name": "name with { braces } and \"quote\"",
    "path": "/project/files/folder\\name",
    "download_url": "https://downloads.sourceforge.net/project/example/folder\\name/file.zip",
    "url": "https://sourceforge.net/projects/example/files/folder%5Cname/file.zip/download",
    "full_path": "/folder\\name/file.zip",
    "type": "f",
    "downloadable": true
  }
}; net.sf.staging_days = 3;
</script>`)

	files, err := ParseFilesPage(body)
	if err != nil {
		t.Fatalf("ParseFilesPage(): %v", err)
	}

	assert.Len(t, files, 1)
	assert.Eq(t, `name with { braces } and "quote"`, files[0].Name)
	assert.Eq(t, `/project/files/folder\name`, files[0].Path)
	assert.Eq(t, `https://downloads.sourceforge.net/project/example/folder\name/file.zip`, files[0].DownloadURL)
	assert.Eq(t, "/folder\\name/file.zip", files[0].FullPath)
	assert.Eq(t, TypeFile, files[0].Type)
	assert.True(t, files[0].Downloadable)
}

func TestParseFilesPageRejectsIncompleteObject(t *testing.T) {
	_, err := ParseFilesPage([]byte(`net.sf.files = {"a":{"name":"x"}`))
	if err == nil || !strings.Contains(err.Error(), "sourceforge files data object is incomplete") {
		t.Fatalf("expected incomplete object error, got %v", err)
	}
}

func TestParseFilesPageRejectsInvalidJSON(t *testing.T) {
	_, err := ParseFilesPage([]byte(`net.sf.files = {"a":{"name":"x" "type":"f"}};`))
	if err == nil || !strings.Contains(err.Error(), "parse sourceforge files data") {
		t.Fatalf("expected parse error, got %v", err)
	}
}
