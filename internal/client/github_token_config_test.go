package client

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestExternalGitHubTokenConfigDisabled(t *testing.T) {
	// a [global] disable_external_token = true setting must stop the gh/git
	// helper resolution the same way the env var does, while an explicitly
	// configured token keeps working untouched
	var helperCalls int
	restore := resolveExternalGitHubToken
	resolveExternalGitHubToken = func() (string, error) {
		helperCalls++
		return "helper-token", nil
	}
	t.Cleanup(func() { resolveExternalGitHubToken = restore })

	token, err := getGitHubToken("", true)

	assert.Eq(t, ErrNoToken, err)
	assert.Eq(t, "", token)
	assert.Eq(t, 0, helperCalls, "helper must not be invoked when disabled by config")
}

func TestExternalGitHubTokenDisabledByEnv(t *testing.T) {
	t.Setenv(DisableExternalTokenEnv, "1")
	var helperCalls int
	restore := resolveExternalGitHubToken
	resolveExternalGitHubToken = func() (string, error) {
		helperCalls++
		return "helper-token", nil
	}
	t.Cleanup(func() { resolveExternalGitHubToken = restore })

	_, err := getGitHubToken("", false)

	assert.Eq(t, ErrNoToken, err)
	assert.Eq(t, 0, helperCalls, "helper must not be invoked when disabled by env")
}
