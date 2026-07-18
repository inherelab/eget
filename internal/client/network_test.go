package client

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestGetWithOptionsSetsDefaultUserAgent(t *testing.T) {
	var gotUA string
	restoreHTTPDo := SetHTTPDoForTest(func(client *http.Client, req *http.Request) (*http.Response, error) {
		gotUA = req.Header.Get("User-Agent")
		return jsonResponse(http.StatusOK, "200 OK", `<html></html>`), nil
	})
	defer restoreHTTPDo()

	resp, err := GetWithOptions("https://example.com/tool.zip", Options{})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	defer resp.Body.Close()

	assert.Eq(t, DefaultUserAgent, gotUA)
	assert.False(t, strings.Contains(gotUA, "Go-http-client"))
}

func TestGetWithOptionsKeepsSourceForgeDownloadUserAgentUnset(t *testing.T) {
	var gotUA string
	restoreHTTPDo := SetHTTPDoForTest(func(client *http.Client, req *http.Request) (*http.Response, error) {
		gotUA = req.Header.Get("User-Agent")
		return jsonResponse(http.StatusOK, "200 OK", `zip body`), nil
	})
	defer restoreHTTPDo()

	resp, err := GetWithOptions("https://downloads.sourceforge.net/project/victoria-ssd-hdd/Victoria537.zip", Options{})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	defer resp.Body.Close()

	assert.Eq(t, "", gotUA)
}

func TestGetWithOptionsDoesNotUseGhproxyForGitHubAPI(t *testing.T) {
	var requested string
	restoreHTTPDo := SetHTTPDoForTest(func(client *http.Client, req *http.Request) (*http.Response, error) {
		requested = req.URL.String()
		return jsonResponse(http.StatusOK, "200 OK", `{}`), nil
	})
	defer restoreHTTPDo()

	resp, err := GetWithOptions("https://api.github.com/repos/gookit/gitw/releases/latest", Options{
		GhproxyEnabled: true,
		GhproxyHostURL: "https://gh.felicity.ac.cn",
	})
	assert.NoErr(t, err)
	_ = resp.Body.Close()

	assert.Eq(t, "https://api.github.com/repos/gookit/gitw/releases/latest", requested)
}

func TestGetWithOptionsUsesConfiguredUserAgent(t *testing.T) {
	var gotUA string
	restoreHTTPDo := SetHTTPDoForTest(func(client *http.Client, req *http.Request) (*http.Response, error) {
		gotUA = req.Header.Get("User-Agent")
		return jsonResponse(http.StatusOK, "200 OK", `<html></html>`), nil
	})
	defer restoreHTTPDo()

	resp, err := GetWithOptions("https://sourceforge.net/projects/victoria-ssd-hdd/files/", Options{UserAgent: "custom-agent/1.0"})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	defer resp.Body.Close()

	assert.Eq(t, "custom-agent/1.0", gotUA)
}

func TestGetWithOptionsRetriesDownloadTransportErrors(t *testing.T) {
	calls := 0
	restore := SetHTTPDoForTest(func(*http.Client, *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, io.ErrUnexpectedEOF
		}
		return jsonResponse(http.StatusOK, "200 OK", "ok"), nil
	})
	defer restore()

	resp, err := GetWithOptions("https://example.test/tool.exe", Options{Retries: 2})
	assert.NoErr(t, err)
	if err != nil {
		return
	}
	assert.Eq(t, 2, calls)
	_ = resp.Body.Close()
}

func TestGetWithOptionsDoesNotRetryProviderMetadata(t *testing.T) {
	calls := 0
	restore := SetHTTPDoForTest(func(*http.Client, *http.Request) (*http.Response, error) {
		calls++
		return nil, io.ErrUnexpectedEOF
	})
	defer restore()

	_, err := GetWithOptions("https://api.github.com/repos/owner/repo/releases/latest", Options{Retries: 3})
	assert.Err(t, err)
	assert.Eq(t, 1, calls)
}

func TestResponseFilename(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{"Content-Disposition": []string{`attachment; filename*=UTF-8''tool%20linux.zip`}},
	}

	assert.Eq(t, "tool linux.zip", responseFilename(resp, "https://example.com/download?id=123"))
	assert.Eq(t, "tool.zip", responseFilename(nil, "https://example.com/artifacts/tool.zip?job=build"))
}
