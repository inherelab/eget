package sdk

import (
	"path/filepath"
	"testing"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
)

func TestServiceSearchIndexMatchesKeywordsAndExcludes(t *testing.T) {
	root := t.TempDir()
	cfg := testSDKConfig(root)
	svc := Service{
		Config:     cfg,
		IndexCache: IndexCache{Dir: filepath.Join(root, "index")},
		GOOS:       "linux",
		GOARCH:     "amd64",
	}
	err := svc.IndexCache.Save(Index{
		Schema: 1,
		SDK:    "go",
		Items: []IndexItem{
			{Version: "1.22.0", Stable: true, Files: []IndexFile{
				{OS: "linux", Arch: "amd64", Ext: "tar.gz", Filename: "go1.22.0.linux-amd64.tar.gz", URL: "https://example.com/go1.22.0.linux-amd64.tar.gz"},
				{OS: "windows", Arch: "amd64", Ext: "zip", Filename: "go1.22.0.windows-amd64.zip", URL: "https://example.com/go1.22.0.windows-amd64.zip"},
			}},
			{Version: "1.22.1-rc.1", Stable: false, Files: []IndexFile{
				{OS: "linux", Arch: "amd64", Ext: "tar.gz", Filename: "go1.22.1-rc.1.linux-amd64.tar.gz", URL: "https://example.com/go1.22.1-rc.1.linux-amd64.tar.gz"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("save index: %v", err)
	}

	results, err := svc.SearchIndex("go", SearchOptions{Keywords: []string{"1.22 amd64", "^windows ^rc"}, Number: 20, IncludePrerelease: true})
	if err != nil {
		t.Fatalf("search index: %v", err)
	}

	assert.Eq(t, 1, len(results))
	assert.Eq(t, "go", results[0].SDK)
	assert.Eq(t, "1.22.0", results[0].Version)
	assert.True(t, results[0].Stable)
	assert.Eq(t, "linux", results[0].OS)
	assert.Eq(t, "amd64", results[0].Arch)
	assert.Eq(t, "go1.22.0.linux-amd64.tar.gz", results[0].Filename)
}

func TestServiceSearchIndexHidesPrereleaseByDefault(t *testing.T) {
	root := t.TempDir()
	cfg := testSDKConfig(root)
	svc := Service{
		Config:     cfg,
		IndexCache: IndexCache{Dir: filepath.Join(root, "index")},
		GOOS:       "linux",
		GOARCH:     "amd64",
	}
	err := svc.IndexCache.Save(Index{
		Schema: 1,
		SDK:    "go",
		Items: []IndexItem{
			{Version: "1.22.0", Stable: true, Files: []IndexFile{
				{OS: "linux", Arch: "amd64", Ext: "tar.gz", Filename: "go1.22.0.linux-amd64.tar.gz"},
			}},
			{Version: "1.23.0-rc.1", Stable: false, Files: []IndexFile{
				{OS: "linux", Arch: "amd64", Ext: "tar.gz", Filename: "go1.23.0-rc.1.linux-amd64.tar.gz"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("save index: %v", err)
	}

	stable, err := svc.SearchIndex("go", SearchOptions{Keywords: []string{"amd64"}, Number: 0})
	if err != nil {
		t.Fatalf("search stable index: %v", err)
	}
	assert.Eq(t, 1, len(stable))
	assert.Eq(t, "1.22.0", stable[0].Version)

	all, err := svc.SearchIndex("go", SearchOptions{Keywords: []string{"amd64"}, Number: 0, IncludePrerelease: true})
	if err != nil {
		t.Fatalf("search all index: %v", err)
	}
	assert.Eq(t, 2, len(all))
}

func TestServiceSearchIndexLimitsResults(t *testing.T) {
	root := t.TempDir()
	cfg := testSDKConfig(root)
	svc := Service{
		Config:     cfg,
		IndexCache: IndexCache{Dir: filepath.Join(root, "index")},
		GOOS:       "linux",
		GOARCH:     "amd64",
	}
	err := svc.IndexCache.Save(Index{
		Schema: 1,
		SDK:    "go",
		Items: []IndexItem{
			{Version: "1.22.0", Stable: true, Files: []IndexFile{
				{OS: "linux", Arch: "amd64", Ext: "tar.gz", Filename: "go1.22.0.linux-amd64.tar.gz"},
				{OS: "darwin", Arch: "amd64", Ext: "tar.gz", Filename: "go1.22.0.darwin-amd64.tar.gz"},
				{OS: "windows", Arch: "amd64", Ext: "zip", Filename: "go1.22.0.windows-amd64.zip"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("save index: %v", err)
	}

	limited, err := svc.SearchIndex("go", SearchOptions{Keywords: []string{"amd64"}, Number: 2})
	if err != nil {
		t.Fatalf("search limited index: %v", err)
	}
	assert.Eq(t, 2, len(limited))

	all, err := svc.SearchIndex("go", SearchOptions{Keywords: []string{"amd64"}, Number: 0})
	if err != nil {
		t.Fatalf("search unlimited index: %v", err)
	}
	assert.Eq(t, 3, len(all))
}

func TestServiceSearchIndexDefaultsToVersionDescSort(t *testing.T) {
	root := t.TempDir()
	cfg := testSDKConfig(root)
	svc := Service{
		Config:     cfg,
		IndexCache: IndexCache{Dir: filepath.Join(root, "index")},
		GOOS:       "linux",
		GOARCH:     "amd64",
	}
	err := svc.IndexCache.Save(Index{
		Schema: 1,
		SDK:    "go",
		Items: []IndexItem{
			{Version: "1.21.13", Stable: true, Files: []IndexFile{{OS: "linux", Arch: "amd64", Ext: "tar.gz", Filename: "go1.21.13.linux-amd64.tar.gz"}}},
			{Version: "1.23.0", Stable: true, Files: []IndexFile{{OS: "linux", Arch: "amd64", Ext: "tar.gz", Filename: "go1.23.0.linux-amd64.tar.gz"}}},
			{Version: "1.22.5", Stable: true, Files: []IndexFile{{OS: "linux", Arch: "amd64", Ext: "tar.gz", Filename: "go1.22.5.linux-amd64.tar.gz"}}},
		},
	})
	if err != nil {
		t.Fatalf("save index: %v", err)
	}

	results, err := svc.SearchIndex("go", SearchOptions{Keywords: []string{"linux amd64"}, Number: 0})
	if err != nil {
		t.Fatalf("search index: %v", err)
	}

	assert.Eq(t, 3, len(results))
	assert.Eq(t, "1.23.0", results[0].Version)
	assert.Eq(t, "1.22.5", results[1].Version)
	assert.Eq(t, "1.21.13", results[2].Version)
}

func TestServiceSearchIndexSupportsRegexAndVersionSort(t *testing.T) {
	root := t.TempDir()
	cfg := testSDKConfig(root)
	cfg.SDK["node"] = cfgpkg.SDKSection{
		Target:          stringPtr("nodejs/node{version}"),
		URLTemplate:     stringPtr("https://example.com/node-v{version}-{os}-{arch}.{ext}"),
		IndexURL:        stringPtr("https://example.com/node/"),
		IndexFormat:     stringPtr("html"),
		FilenamePattern: stringPtr("node-v{version}-{os}-{arch}.{ext}"),
		ExtMap:          map[string]string{"linux": "tar.gz"},
		ArchMap:         map[string]string{"amd64": "x64"},
	}
	svc := Service{
		Config:     cfg,
		IndexCache: IndexCache{Dir: filepath.Join(root, "index")},
		GOOS:       "linux",
		GOARCH:     "amd64",
	}
	err := svc.IndexCache.Save(Index{
		Schema: 1,
		SDK:    "node",
		Items: []IndexItem{
			{Version: "20.11.1", Stable: true, Files: []IndexFile{{OS: "linux", Arch: "amd64", Ext: "tar.gz", Filename: "node-v20.11.1-linux-x64.tar.gz"}}},
			{Version: "22.0.0", Stable: true, Files: []IndexFile{{OS: "linux", Arch: "amd64", Ext: "tar.gz", Filename: "node-v22.0.0-linux-x64.tar.gz"}}},
			{Version: "22.3.1", Stable: true, Files: []IndexFile{{OS: "linux", Arch: "amd64", Ext: "tar.gz", Filename: "node-v22.3.1-linux-x64.tar.gz"}}},
		},
	})
	if err != nil {
		t.Fatalf("save index: %v", err)
	}

	desc, err := svc.SearchIndex("node", SearchOptions{Keywords: []string{"REG:^22"}, Sort: "desc", Number: 1})
	if err != nil {
		t.Fatalf("search desc index: %v", err)
	}
	assert.Eq(t, 1, len(desc))
	assert.Eq(t, "22.3.1", desc[0].Version)

	asc, err := svc.SearchIndex("node", SearchOptions{Keywords: []string{"REG:^22"}, Sort: "asc", Number: 0})
	if err != nil {
		t.Fatalf("search asc index: %v", err)
	}
	assert.Eq(t, 2, len(asc))
	assert.Eq(t, "22.0.0", asc[0].Version)
	assert.Eq(t, "22.3.1", asc[1].Version)

	_, err = svc.SearchIndex("node", SearchOptions{Keywords: []string{"REG:["}})
	assert.Err(t, err)
}
