package sdk

import (
	"strings"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
)

func TestParseJSONIndexForGo(t *testing.T) {
	body := strings.NewReader(`[
  {
    "version": "go1.21.1",
    "stable": true,
    "files": [
      {"filename": "go1.21.1.windows-amd64.zip", "os": "windows", "arch": "amd64", "version": "go1.21.1", "sha256": "abc", "size": 123, "kind": "archive"},
      {"filename": "go1.21.1.linux-amd64.tar.gz", "os": "linux", "arch": "amd64", "version": "go1.21.1", "sha256": "def", "size": 456, "kind": "archive"}
    ]
  }
]`)
	now := time.Date(2026, 5, 17, 11, 0, 0, 0, time.UTC)

	index, err := ParseJSONIndex(body, "go-json", JSONParseOptions{
		SDK:       "go",
		SourceURL: "https://go.dev/dl/?mode=json",
		Now:       func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("parse json index: %v", err)
	}

	assert.Eq(t, 1, index.Schema)
	assert.Eq(t, "go", index.SDK)
	assert.Eq(t, now, index.FetchedAt)
	assert.Eq(t, "1.21.1", index.Items[0].Version)
	assert.True(t, index.Items[0].Stable)
	assert.Eq(t, "tar.gz", indexFileByOS(t, index.Items[0].Files, "linux").Ext)
	assert.Eq(t, "https://go.dev/dl/go1.21.1.linux-amd64.tar.gz", indexFileByOS(t, index.Items[0].Files, "linux").URL)
}

func TestParseJSONIndexForNode(t *testing.T) {
	// The descriptors are taken verbatim from nodejs.org/dist/index.json; a
	// descriptor names a platform, not a file (see parseNodeFileDescriptor).
	body := strings.NewReader(`[
  {
    "version": "v20.11.1",
    "date": "2024-02-13",
    "files": ["aix-ppc64", "headers", "linux-arm64", "linux-x64", "linux-x64-musl",
              "osx-arm64-tar", "osx-x64-pkg", "osx-x64-tar", "src",
              "win-arm64-7z", "win-arm64-zip", "win-x64-exe", "win-x64-msi", "win-x64-zip"]
  }
]`)

	index, err := ParseJSONIndex(body, "node-json", JSONParseOptions{
		SDK:       "node",
		SourceURL: "https://nodejs.org/dist/index.json",
	})
	if err != nil {
		t.Fatalf("parse json index: %v", err)
	}

	assert.Eq(t, "20.11.1", index.Items[0].Version)
	files := index.Items[0].Files

	winZip := indexFileByOSExt(t, files, "win-x64", "zip")
	assert.Eq(t, "https://nodejs.org/dist/v20.11.1/node-v20.11.1-win-x64.zip", winZip.URL)
	assert.Eq(t, "7z", indexFileByOSExt(t, files, "win-arm64", "7z").Ext)

	// A bare descriptor expands to the tarballs the dist publishes.
	assert.Eq(t, "https://nodejs.org/dist/v20.11.1/node-v20.11.1-linux-x64.tar.gz",
		indexFileByOSExt(t, files, "linux-x64", "tar.gz").URL)
	assert.Eq(t, "https://nodejs.org/dist/v20.11.1/node-v20.11.1-linux-x64.tar.xz",
		indexFileByOSExt(t, files, "linux-x64", "tar.xz").URL)
	assert.Eq(t, "https://nodejs.org/dist/v20.11.1/node-v20.11.1-linux-x64-musl.tar.gz",
		indexFileByOSExt(t, files, "linux-x64-musl", "tar.gz").URL)

	// "osx-x64-tar" is published under the darwin name.
	darwin := indexFileByOSExt(t, files, "darwin-x64", "tar.gz")
	assert.Eq(t, "https://nodejs.org/dist/v20.11.1/node-v20.11.1-darwin-x64.tar.gz", darwin.URL)
	assert.Eq(t, "tar.xz", indexFileByOSExt(t, files, "darwin-arm64", "tar.xz").Ext)

	// aix publishes only .tar.gz; headers/src and the installers are not archives.
	assert.Eq(t, "tar.gz", indexFileByOSExt(t, files, "aix-ppc64", "tar.gz").Ext)
	for _, entry := range files {
		assert.NotEq(t, "aix-ppc64/tar.xz", entry.OS+"-"+entry.Arch+"/"+entry.Ext)
		assert.False(t, entry.Ext == "exe" || entry.Ext == "msi" || entry.Ext == "pkg", entry.Filename)
		assert.False(t, strings.Contains(entry.Filename, "headers") || entry.Filename == "node-v20.11.1.tar.gz", entry.Filename)
	}
}

// indexFileByOSExt finds one entry by its os-arch pair and extension, which is
// what a node-json descriptor expands into.
func indexFileByOSExt(t *testing.T, files []IndexFile, osArch, ext string) IndexFile {
	t.Helper()
	for _, file := range files {
		if file.OS+"-"+file.Arch == osArch && file.Ext == ext {
			return file
		}
	}
	t.Fatalf("expected file for %s .%s in %#v", osArch, ext, files)
	return IndexFile{}
}

func TestParseJSONIndexForZulu(t *testing.T) {
	body := strings.NewReader(`[
  {
    "availability_type": "CA",
    "distro_version": [21, 44, 17, 0],
    "download_url": "https://cdn.azul.com/zulu/bin/zulu21.44.17-ca-jdk21.0.8-linux_x64.tar.gz",
    "java_version": [21, 0, 8],
    "latest": true,
    "name": "zulu21.44.17-ca-jdk21.0.8-linux_x64.tar.gz",
    "openjdk_build_number": 9,
    "product": "zulu"
  },
  {
    "availability_type": "CA",
    "distro_version": [21, 44, 17, 0],
    "download_url": "https://cdn.azul.com/zulu/bin/zulu21.44.17-ca-jdk21.0.8-win_x64.zip",
    "java_version": [21, 0, 8],
    "latest": true,
    "name": "zulu21.44.17-ca-jdk21.0.8-win_x64.zip",
    "openjdk_build_number": 9,
    "product": "zulu"
  }
]`)
	now := time.Date(2026, 5, 22, 21, 0, 0, 0, time.UTC)

	index, err := ParseJSONIndex(body, "zulu-json", JSONParseOptions{
		SDK:       "jdk",
		SourceURL: "https://api.azul.com/metadata/v1/zulu/packages",
		Now:       func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("parse zulu json index: %v", err)
	}

	assert.Eq(t, "jdk", index.SDK)
	assert.Eq(t, now, index.FetchedAt)
	assert.Eq(t, 1, len(index.Items))
	assert.Eq(t, "21.0.8", index.Items[0].Version)
	assert.True(t, index.Items[0].Stable)
	linuxFile := indexFileByOS(t, index.Items[0].Files, "linux")
	assert.Eq(t, "x64", linuxFile.Arch)
	assert.Eq(t, "tar.gz", linuxFile.Ext)
	assert.Eq(t, "https://cdn.azul.com/zulu/bin/zulu21.44.17-ca-jdk21.0.8-linux_x64.tar.gz", linuxFile.URL)
	winFile := indexFileByOS(t, index.Items[0].Files, "win")
	assert.Eq(t, "x64", winFile.Arch)
	assert.Eq(t, "zip", winFile.Ext)
}

func TestParseJSONIndexForZuluSkipsVariantPackages(t *testing.T) {
	body := strings.NewReader(`[
  {
    "download_url": "https://cdn.azul.com/zulu/bin/zulu21.44.17-ca-jdk21.0.8-linux_x64.tar.gz",
    "java_version": [21, 0, 8],
    "name": "zulu21.44.17-ca-jdk21.0.8-linux_x64.tar.gz"
  },
  {
    "download_url": "https://cdn.azul.com/zulu/bin/zulu21.44.17-ca-fx-jdk21.0.8-linux_x64.tar.gz",
    "java_version": [21, 0, 8],
    "name": "zulu21.44.17-ca-fx-jdk21.0.8-linux_x64.tar.gz"
  },
  {
    "download_url": "https://cdn.azul.com/zulu/bin/zulu21.44.17-ca-crac-jdk21.0.8-win_x64.zip",
    "java_version": [21, 0, 8],
    "name": "zulu21.44.17-ca-crac-jdk21.0.8-win_x64.zip"
  }
]`)

	index, err := ParseJSONIndex(body, "zulu-json", JSONParseOptions{SDK: "jdk"})
	if err != nil {
		t.Fatalf("parse zulu json index: %v", err)
	}

	assert.Eq(t, 1, len(index.Items))
	assert.Eq(t, 1, len(index.Items[0].Files))
	assert.Eq(t, "zulu21.44.17-ca-jdk21.0.8-linux_x64.tar.gz", index.Items[0].Files[0].Filename)
}

func TestParseJSONIndexRejectsUnsupportedParser(t *testing.T) {
	_, err := ParseJSONIndex(strings.NewReader(`[]`), "unknown", JSONParseOptions{})
	assert.Err(t, err)
}
