package sourceforge

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gookit/goutil/testutil/assert"
)

type fakeGetter struct {
	responses map[string]string
	requests  []string
}

func (g *fakeGetter) Get(url string) (*http.Response, error) {
	g.requests = append(g.requests, url)
	return htmlResponse(g.responses[url]), nil
}

func htmlResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestFinderFindsLatestFilesUnderSourcePath(t *testing.T) {
	firstURL := "https://sourceforge.net/projects/winmerge/files/stable/"
	secondURL := "https://sourceforge.net/projects/winmerge/files/stable/2.16.44/"
	getter := &fakeGetter{responses: map[string]string{
		firstURL: `
<script>
net.sf.files = {
  "2.16.42": {"name":"2.16.42","full_path":"/stable/2.16.42","type":"d"},
  "2.16.44": {"name":"2.16.44","full_path":"/stable/2.16.44","type":"d"}
};
</script>`,
		secondURL: `
<script>
net.sf.files = {
  "WinMerge-2.16.44-x64-Setup.exe": {
    "name":"WinMerge-2.16.44-x64-Setup.exe",
    "download_url":"https://downloads.sourceforge.net/project/winmerge/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe",
    "url":"https://sourceforge.net/projects/winmerge/files/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe/download",
    "full_path":"/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe",
    "type":"f"
  }
};
</script>`,
	}}

	urls, err := Finder{Project: "winmerge", Path: "stable", Getter: getter}.Find()

	if err != nil {
		t.Fatalf("Find(): %v", err)
	}
	assert.Len(t, urls, 1)
	assert.Eq(t, "https://downloads.sourceforge.net/project/winmerge/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe", urls[0])
	assert.Len(t, getter.requests, 2)
}

func TestFinderNormalizesSourceForgeDownloadPageURL(t *testing.T) {
	getter := &fakeGetter{responses: map[string]string{
		"https://sourceforge.net/projects/winmerge/files/stable/2.16.56/": `
<script>
net.sf.files = {
  "WinMerge-2.16.56-x64-Setup.exe": {
    "name":"WinMerge-2.16.56-x64-Setup.exe",
    "download_url":"https://sourceforge.net/projects/winmerge/files/stable/2.16.56/WinMerge-2.16.56-x64-Setup.exe/download",
    "url":"/projects/winmerge/files/stable/2.16.56/WinMerge-2.16.56-x64-Setup.exe/",
    "full_path":"/stable/2.16.56/WinMerge-2.16.56-x64-Setup.exe",
    "type":"f"
  }
};
</script>`,
	}}

	urls, err := Finder{Project: "winmerge", Path: "stable/2.16.56", Getter: getter}.Find()

	assert.NoErr(t, err)
	assert.Eq(t, []string{"https://downloads.sourceforge.net/project/winmerge/stable/2.16.56/WinMerge-2.16.56-x64-Setup.exe"}, urls)
}

func TestFinderEscapesSourcePathURLSegments(t *testing.T) {
	getter := &fakeGetter{responses: map[string]string{
		"https://sourceforge.net/projects/keepass/files/Translations%202.x/2.59/": `
<script>
net.sf.files = {
  "Ukrainian.zip": {
    "name":"Ukrainian.zip",
    "download_url":"https://downloads.sourceforge.net/project/keepass/Translations%202.x/2.59/Ukrainian.zip",
    "full_path":"/Translations 2.x/2.59/Ukrainian.zip",
    "type":"f"
  }
};
</script>`,
	}}

	_, err := Finder{Project: "keepass", Path: "Translations 2.x", Tag: "2.59", Getter: getter}.Find()

	assert.NoErr(t, err)
	assert.Eq(t, []string{"https://sourceforge.net/projects/keepass/files/Translations%202.x/2.59/"}, getter.requests)
}

func TestFinderWithoutPathPrefersStableDirectory(t *testing.T) {
	rootURL := "https://sourceforge.net/projects/winmerge/files/"
	stableURL := "https://sourceforge.net/projects/winmerge/files/stable/"
	versionURL := "https://sourceforge.net/projects/winmerge/files/stable/2.16.44/"
	getter := &fakeGetter{responses: map[string]string{
		rootURL: `
<script>
net.sf.files = {
  "beta": {"name":"beta","full_path":"/beta","type":"d"},
  "stable": {"name":"stable","full_path":"/stable","type":"d"}
};
</script>`,
		stableURL: `
<script>
net.sf.files = {
  "2.16.42": {"name":"2.16.42","full_path":"/stable/2.16.42","type":"d"},
  "2.16.44": {"name":"2.16.44","full_path":"/stable/2.16.44","type":"d"}
};
</script>`,
		versionURL: `
<script>
net.sf.files = {
  "WinMerge-2.16.44-x64-Setup.exe": {
    "name":"WinMerge-2.16.44-x64-Setup.exe",
    "download_url":"https://downloads.sourceforge.net/project/winmerge/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe",
    "full_path":"/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe",
    "type":"f"
  }
};
</script>`,
	}}

	urls, err := Finder{Project: "winmerge", Getter: getter}.Find()

	assert.NoErr(t, err)
	assert.Eq(t, []string{"https://downloads.sourceforge.net/project/winmerge/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe"}, urls)
	assert.Eq(t, []string{rootURL, stableURL, versionURL}, getter.requests)
}

func TestLatestVersionUsesSourcePath(t *testing.T) {
	getter := &fakeGetter{responses: map[string]string{
		"https://sourceforge.net/projects/winmerge/files/stable/": `
<script>
net.sf.files = {
  "2.16.42": {"name":"2.16.42","full_path":"/stable/2.16.42","type":"d"},
  "2.16.44": {"name":"2.16.44","full_path":"/stable/2.16.44","type":"d"}
};
</script>`,
	}}

	info, err := LatestVersion("winmerge", "stable", getter)

	assert.NoErr(t, err)
	assert.Eq(t, []string{"https://sourceforge.net/projects/winmerge/files/stable/"}, getter.requests)
	assert.Eq(t, "2.16.44", info.Version)
	assert.Eq(t, "/stable/2.16.44", info.Path)
}

func TestFinderFallbackVersionAssetsSkipsLatestAndScansOlderVersions(t *testing.T) {
	baseURL := "https://sourceforge.net/projects/keepass/files/Translations%202.x/"
	version260URL := "https://sourceforge.net/projects/keepass/files/Translations%202.x/2.60/"
	version259URL := "https://sourceforge.net/projects/keepass/files/Translations%202.x/2.59/"
	getter := &fakeGetter{responses: map[string]string{
		baseURL: `
<script>
net.sf.files = {
  "2.59": {"name":"2.59","full_path":"/Translations 2.x/2.59","type":"d"},
  "2.60": {"name":"2.60","full_path":"/Translations 2.x/2.60","type":"d"},
  "2.61": {"name":"2.61","full_path":"/Translations 2.x/2.61","type":"d"}
};
</script>`,
		version260URL: `
<script>
net.sf.files = {
  "Spanish.zip": {
    "name":"Spanish.zip",
    "download_url":"https://downloads.sourceforge.net/project/keepass/Translations%202.x/2.60/Spanish.zip",
    "full_path":"/Translations 2.x/2.60/Spanish.zip",
    "type":"f"
  }
};
</script>`,
		version259URL: `
<script>
net.sf.files = {
  "Ukrainian.zip": {
    "name":"Ukrainian.zip",
    "download_url":"https://downloads.sourceforge.net/project/keepass/Translations%202.x/2.59/Ukrainian.zip",
    "full_path":"/Translations 2.x/2.59/Ukrainian.zip",
    "type":"f"
  }
};
</script>`,
	}}

	assets, err := Finder{Project: "keepass", Path: "Translations 2.x", Getter: getter}.FallbackVersionAssets(2)

	assert.NoErr(t, err)
	assert.Eq(t, [][]string{
		{"https://downloads.sourceforge.net/project/keepass/Translations%202.x/2.60/Spanish.zip"},
		{"https://downloads.sourceforge.net/project/keepass/Translations%202.x/2.59/Ukrainian.zip"},
	}, assets)
	assert.Eq(t, []string{baseURL, version260URL, version259URL}, getter.requests)
}

func TestFinderFallbackVersionAssetsHonorsLimit(t *testing.T) {
	baseURL := "https://sourceforge.net/projects/keepass/files/Translations%202.x/"
	version260URL := "https://sourceforge.net/projects/keepass/files/Translations%202.x/2.60/"
	getter := &fakeGetter{responses: map[string]string{
		baseURL: `
<script>
net.sf.files = {
  "2.59": {"name":"2.59","full_path":"/Translations 2.x/2.59","type":"d"},
  "2.60": {"name":"2.60","full_path":"/Translations 2.x/2.60","type":"d"},
  "2.61": {"name":"2.61","full_path":"/Translations 2.x/2.61","type":"d"}
};
</script>`,
		version260URL: `
<script>
net.sf.files = {
  "Spanish.zip": {
    "name":"Spanish.zip",
    "download_url":"https://downloads.sourceforge.net/project/keepass/Translations%202.x/2.60/Spanish.zip",
    "full_path":"/Translations 2.x/2.60/Spanish.zip",
    "type":"f"
  }
};
</script>`,
	}}

	assets, err := Finder{Project: "keepass", Path: "Translations 2.x", Getter: getter}.FallbackVersionAssets(1)

	assert.NoErr(t, err)
	assert.Eq(t, [][]string{{"https://downloads.sourceforge.net/project/keepass/Translations%202.x/2.60/Spanish.zip"}}, assets)
	assert.Eq(t, []string{baseURL, version260URL}, getter.requests)
}
