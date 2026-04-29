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
