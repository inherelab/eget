package client

import (
	"net/http"
	"os"
	"os/exec"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestSetAuthHeaderTokenPrecedence(t *testing.T) {
	apiURL := "https://api.github.com/repos/x/y"
	newReq := func() *http.Request {
		r, err := http.NewRequest(http.MethodGet, apiURL, nil)
		assert.NoErr(t, err)
		return r
	}

	t.Run("explicit option wins and skips external helpers", func(t *testing.T) {
		origResolve := resolveExternalGitHubToken
		defer func() { resolveExternalGitHubToken = origResolve }()
		resolveExternalGitHubToken = func() (string, error) {
			t.Fatal("external helpers must not run while an explicit token is set")
			return "", nil
		}
		t.Setenv("EGET_GITHUB_TOKEN", "")
		t.Setenv("GITHUB_TOKEN", "")

		r := newReq()
		assert.NoErr(t, setAuthHeader(r, Options{GithubToken: "opt-token"}))
		assert.Eq(t, "token opt-token", r.Header.Get("Authorization"))
	})

	t.Run("env token used without option", func(t *testing.T) {
		t.Setenv("EGET_GITHUB_TOKEN", "")
		t.Setenv("GITHUB_TOKEN", "env-token")
		origResolve := resolveExternalGitHubToken
		defer func() { resolveExternalGitHubToken = origResolve }()
		resolveExternalGitHubToken = func() (string, error) {
			t.Fatal("external helpers must not run while a env token is set")
			return "", nil
		}

		r := newReq()
		assert.NoErr(t, setAuthHeader(r, Options{}))
		assert.Eq(t, "token env-token", r.Header.Get("Authorization"))
	})

	t.Run("external helper fallback when no explicit token", func(t *testing.T) {
		t.Setenv("EGET_GITHUB_TOKEN", "")
		t.Setenv("GITHUB_TOKEN", "")
		origResolve := resolveExternalGitHubToken
		defer func() { resolveExternalGitHubToken = origResolve }()
		resolveExternalGitHubToken = func() (string, error) { return "helper-token", nil }

		r := newReq()
		assert.NoErr(t, setAuthHeader(r, Options{}))
		assert.Eq(t, "token helper-token", r.Header.Get("Authorization"))
	})

	t.Run("empty external result keeps anonymous", func(t *testing.T) {
		t.Setenv("EGET_GITHUB_TOKEN", "")
		t.Setenv("GITHUB_TOKEN", "")
		origResolve := resolveExternalGitHubToken
		defer func() { resolveExternalGitHubToken = origResolve }()
		resolveExternalGitHubToken = func() (string, error) { return "", ErrNoToken }

		r := newReq()
		assert.NoErr(t, setAuthHeader(r, Options{}))
		assert.Eq(t, "", r.Header.Get("Authorization"))
	})

	t.Run("non api host gets no auth header", func(t *testing.T) {
		r, err := http.NewRequest(http.MethodGet, "https://example.com/file.zip", nil)
		assert.NoErr(t, err)
		assert.NoErr(t, setAuthHeader(r, Options{GithubToken: "opt-token"}))
		assert.Eq(t, "", r.Header.Get("Authorization"))
	})
}

func TestGitCredentialPasswordParsing(t *testing.T) {
	out := "protocol=https\nhost=github.com\nusername=inhere\npassword=gho_abc\n"
	assert.Eq(t, "gho_abc", gitCredentialPassword(out))
	assert.Eq(t, "", gitCredentialPassword("protocol=https\nhost=github.com\n"))
}

// TestExternalGitHubTokenOnThisMachine exercises the real helpers when the tools
// exist *and* an opt-in is given. Without the opt-in it skips: hosted runners
// ship gh without credentials, so probing it there always fails.
func TestExternalGitHubTokenOnThisMachine(t *testing.T) {
	if os.Getenv("EGET_TEST_EXTERNAL") == "" {
		t.Skip("set EGET_TEST_EXTERNAL=1 to probe the local gh credentials")
	}
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not installed")
	}
	token := ghAuthTokenOutput()
	if token == "" {
		t.Skip("gh has no stored token on this machine")
	}
	assert.True(t, len(token) >= 20)
}

func TestEnvTokenFromFileSyntax(t *testing.T) {
	dir := t.TempDir()
	file := dir + "/token.txt"
	assert.NoErr(t, os.WriteFile(file, []byte("file-token\r\n"), 0o600))

	t.Setenv("EGET_GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "@"+file)
	r, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/x/y", nil)
	assert.NoErr(t, err)

	assert.NoErr(t, setAuthHeader(r, Options{}))
	assert.Eq(t, "token file-token", r.Header.Get("Authorization"))
}
