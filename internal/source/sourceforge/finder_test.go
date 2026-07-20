package sourceforge

import (
	"net/http"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func (f fakeHTTPGetterFunc) Get(url string) (*http.Response, error) {
	return f(url)
}

func (g *fakeGetter) Get(url string) (*http.Response, error) {
	g.requests = append(g.requests, url)
	return htmlResponse(g.responses[url]), nil
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

func TestFinderPrioritizesNewestFilesInSameDirectory(t *testing.T) {
	rootURL := "https://sourceforge.net/projects/windjviewextended/files/"
	getter := &fakeGetter{responses: map[string]string{
		rootURL: `
<script>
net.sf.files = {
  "WinDjView Extended 4.0.1.zip": {
    "name":"WinDjView Extended 4.0.1.zip",
    "download_url":"https://downloads.sourceforge.net/project/windjviewextended/WinDjView%20Extended%204.0.1.zip",
    "type":"f"
  },
  "WinDjView Extended 4.1.zip": {
    "name":"WinDjView Extended 4.1.zip",
    "download_url":"https://downloads.sourceforge.net/project/windjviewextended/WinDjView%20Extended%204.1.zip",
    "type":"f"
  }
};
</script>`,
	}}

	urls, err := Finder{Project: "windjviewextended", Getter: getter}.Find()

	assert.NoErr(t, err)
	assert.Len(t, urls, 2)
	assert.Contains(t, urls[0], "4.1.zip")
	assert.Contains(t, urls[1], "4.0.1.zip")
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

func TestFinderFallsBackToRSSWhenFilesPageIsForbidden(t *testing.T) {
	getter := fakeHTTPGetterFunc(func(rawURL string) (*http.Response, error) {
		switch rawURL {
		case "https://sourceforge.net/projects/victoria-ssd-hdd/files/":
			return statusResponse(http.StatusForbidden, "cf challenge"), nil
		case "https://sourceforge.net/projects/victoria-ssd-hdd/rss?path=/":
			return htmlResponse(`<?xml version="1.0" encoding="utf-8"?>
<rss xmlns:media="http://video.search.yahoo.com/mrss/" version="2.0">
  <channel>
    <item>
      <title><![CDATA[/Victoria537.zip]]></title>
      <link>https://sourceforge.net/projects/victoria-ssd-hdd/files/Victoria537.zip/download</link>
      <pubDate>Mon, 20 Oct 2025 01:21:41 UT</pubDate>
      <media:content url="https://sourceforge.net/projects/victoria-ssd-hdd/files/Victoria537.zip/download" />
    </item>
  </channel>
</rss>`), nil
		default:
			t.Fatalf("unexpected request %s", rawURL)
			return nil, nil
		}
	})

	urls, err := Finder{Project: "victoria-ssd-hdd", Getter: getter}.Find()

	assert.NoErr(t, err)
	assert.Eq(t, []string{"https://downloads.sourceforge.net/project/victoria-ssd-hdd/Victoria537.zip"}, urls)
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
