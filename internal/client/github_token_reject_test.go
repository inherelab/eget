package client

import (
	"net/http"
	"sync"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

// resetExternalTokenState clears the per-process resolution/rejection state so a
// test can drive the fallback deterministically.
func resetExternalTokenState() {
	externalResolveOnce = sync.Once{}
	externalResolvedToken = ""
	externalTokenRejected.Store(false)
	externalTokenWarnOnce = sync.Once{}
}

func clearTokenEnv(t *testing.T) {
	t.Helper()
	t.Setenv("EGET_GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv(DisableExternalTokenEnv, "")
}

func apiRequest(t *testing.T, opts Options) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/x/y", nil)
	assert.NoErr(t, err)
	assert.NoErr(t, setAuthHeader(req, opts))
	return req
}

// TestExternalTokenDisabledByEnv pins the opt-out switch: with it set, the local
// gh/git logins are never consulted (issue #62 escape hatch).
func TestExternalTokenDisabledByEnv(t *testing.T) {
	resetExternalTokenState()
	defer resetExternalTokenState()
	clearTokenEnv(t)
	t.Setenv(DisableExternalTokenEnv, "1")

	origResolve := resolveExternalGitHubToken
	defer func() { resolveExternalGitHubToken = origResolve }()
	resolveExternalGitHubToken = func() (string, error) {
		t.Fatal("local logins must not be consulted while disabled")
		return "", nil
	}

	assert.Eq(t, "", apiRequest(t, Options{}).Header.Get("Authorization"))
}

// TestUnauthorizedExternalTokenFallsBackToAnonymous is the issue #62 regression
// guard: a stale local login (401 Bad credentials) must not break a request that
// works anonymously, and must not be reused afterwards.
func TestUnauthorizedExternalTokenFallsBackToAnonymous(t *testing.T) {
	resetExternalTokenState()
	defer resetExternalTokenState()
	clearTokenEnv(t)

	origResolve := resolveExternalGitHubToken
	defer func() { resolveExternalGitHubToken = origResolve }()
	resolveExternalGitHubToken = func() (string, error) { return "stale-token", nil }

	origHTTPDo := httpDo
	defer func() { httpDo = origHTTPDo }()
	var authHeaders []string
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		header := req.Header.Get("Authorization")
		authHeaders = append(authHeaders, header)
		if header != "" {
			return jsonResponse(http.StatusUnauthorized, "401 Unauthorized",
				`{"message":"Bad credentials","documentation_url":"https://docs.github.com/rest","status":"401"}`), nil
		}
		return jsonResponse(http.StatusOK, "200 OK", `{"tag_name":"v1.0.0"}`), nil
	}

	newRequest := func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/sharkdp/bat/releases/latest", nil)
		if err != nil {
			return nil, err
		}
		if err := setAuthHeader(req, Options{}); err != nil {
			return nil, err
		}
		return req, nil
	}

	resp, err := doRequestWithRetries(http.DefaultClient, 1, newRequest)
	assert.NoErr(t, err)
	assert.Eq(t, http.StatusOK, resp.StatusCode)
	assert.Eq(t, []string{"token stale-token", ""}, authHeaders)

	// The rejected token is dropped for the rest of the process.
	assert.Eq(t, "", apiRequest(t, Options{}).Header.Get("Authorization"))
}

// TestUserConfiguredTokenIsNotRetried: an explicit/env token is the user's own
// credential, so its 401 stays visible instead of silently going anonymous.
func TestUserConfiguredTokenIsNotRetried(t *testing.T) {
	resetExternalTokenState()
	defer resetExternalTokenState()
	clearTokenEnv(t)

	origResolve := resolveExternalGitHubToken
	defer func() { resolveExternalGitHubToken = origResolve }()
	resolveExternalGitHubToken = func() (string, error) { return "stale-token", nil }

	origHTTPDo := httpDo
	defer func() { httpDo = origHTTPDo }()
	calls := 0
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(http.StatusUnauthorized, "401 Unauthorized", `{"message":"Bad credentials","status":"401"}`), nil
	}

	newRequest := func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/x/y", nil)
		if err != nil {
			return nil, err
		}
		if err := setAuthHeader(req, Options{GithubToken: "user-token"}); err != nil {
			return nil, err
		}
		return req, nil
	}

	resp, err := doRequestWithRetries(http.DefaultClient, 1, newRequest)
	assert.NoErr(t, err)
	assert.Eq(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Eq(t, 1, calls)
}
