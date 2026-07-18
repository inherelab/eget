package client

import (
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestRequestWithOptionsRetriesTransportErrors(t *testing.T) {
	calls := 0
	restore := SetHTTPDoForTest(func(*http.Client, *http.Request) (*http.Response, error) {
		calls++
		if calls < 3 {
			return nil, io.ErrUnexpectedEOF
		}
		return jsonResponse(http.StatusOK, "200 OK", "ok"), nil
	})
	defer restore()

	resp, err := requestWithOptions(http.MethodGet, "https://example.test/tool", "", Options{Retries: 3})
	assert.NoErr(t, err)
	if err != nil {
		return
	}
	assert.Eq(t, http.StatusOK, resp.StatusCode)
	assert.Eq(t, 3, calls)
	_ = resp.Body.Close()
}

func TestRequestWithOptionsReturnsLastRetryError(t *testing.T) {
	calls := 0
	restore := SetHTTPDoForTest(func(*http.Client, *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("first attempt")
		}
		return nil, errors.New("last attempt")
	})
	defer restore()

	_, err := requestWithOptions(http.MethodGet, "https://example.test/tool", "", Options{Retries: 2})
	assert.Err(t, err)
	assert.Contains(t, err.Error(), "last attempt")
	assert.Eq(t, 2, calls)
}

func TestRequestWithOptionsRetriesBeforeGhproxyFallback(t *testing.T) {
	var hosts []string
	restore := SetHTTPDoForTest(func(_ *http.Client, req *http.Request) (*http.Response, error) {
		hosts = append(hosts, req.URL.Host)
		return nil, errors.New("unavailable")
	})
	defer restore()

	_, err := requestWithOptions(http.MethodGet, "https://github.com/owner/repo/releases/download/v1/tool.zip", "", Options{
		Retries:          2,
		GhproxyEnabled:   true,
		GhproxyHostURL:   "https://primary.proxy.test",
		GhproxyFallbacks: []string{"https://fallback.proxy.test"},
	})

	assert.Err(t, err)
	assert.Eq(t, []string{"primary.proxy.test", "primary.proxy.test", "fallback.proxy.test", "fallback.proxy.test"}, hosts)
}
