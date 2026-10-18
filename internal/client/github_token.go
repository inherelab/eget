package client

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// externalTokenRunTimeout bounds each helper invocation so a misbehaving
// helper cannot stall requests.
const externalTokenRunTimeout = 5 * time.Second

// DisableExternalTokenEnv turns off reusing the local gh/git logins.
const DisableExternalTokenEnv = "EGET_DISABLE_EXTERNAL_TOKEN"

// externalGitHubToken resolves a token from local tool logins without any
// eget-specific configuration, so api.github.com requests skip the anonymous
// 60 requests/hour rate limit when gh or git is already authenticated:
//
//	gh: "gh auth token" (the official accessor; gh may keep the token in the
//	    OS keyring, where hosts.yml has none)
//	git: "git credential fill" for https://github.com, which queries the
//	     configured credential helper (e.g. Git Credential Manager)
//
// The resolution runs at most once per process and failures stay cached, so
// requests never pay the helper cost twice. Tokens are never logged; failure
// is silent and leaves callers anonymous, preserving the old behavior.
//
// A token GitHub rejects with 401 is dropped for the rest of the process (see
// rejectUnauthorizedExternalToken): a stale local login must not break requests
// that work anonymously.
func externalGitHubToken() (string, error) {
	if externalTokensDisabled() || externalTokenRejected.Load() {
		return "", ErrNoToken
	}
	token, err := resolveExternalGitHubToken()
	if err != nil || token == "" {
		return "", ErrNoToken
	}
	return token, nil
}

// externalTokensDisabled reports whether the user opted out of reusing local
// gh/git logins.
func externalTokensDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(DisableExternalTokenEnv))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// rejectUnauthorizedExternalToken reports whether req carried the externally
// discovered token. It then marks that token rejected (once per process, with a
// single warning) so the caller can retry the request anonymously and later
// requests stop paying the round trip.
//
// A token the user configured explicitly is never rejected here: a 401 on it is
// the user's own credential problem and must keep failing loudly.
func rejectUnauthorizedExternalToken(req *http.Request) bool {
	if req == nil || externalTokenRejected.Load() {
		return false
	}
	header := req.Header.Get("Authorization")
	if header == "" {
		return false
	}
	token, err := resolveExternalGitHubToken()
	if err != nil || token == "" || header != "token "+token {
		return false
	}

	externalTokenRejected.Store(true)
	externalTokenWarnOnce.Do(func() {
		fmt.Fprintf(os.Stderr, "warning: the GitHub token from the local gh/git login was rejected (401 Bad credentials); retrying anonymously. Set %s=1 to stop reusing local logins.\n", DisableExternalTokenEnv)
	})
	return true
}

var (
	resolveExternalGitHubToken = resolveExternalGitHubTokenOnce
	externalResolveOnce        sync.Once
	externalResolvedToken      string
	externalTokenRejected      atomic.Bool
	externalTokenWarnOnce      sync.Once
)

func resolveExternalGitHubTokenOnce() (string, error) {
	externalResolveOnce.Do(func() {
		if token := strings.TrimSpace(ghAuthTokenOutput()); token != "" {
			externalResolvedToken = token
			return
		}
		externalResolvedToken = gitCredentialToken()
	})
	return externalResolvedToken, nil
}

func ghAuthTokenOutput() string {
	ctx, cancel := helperContext()
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "auth", "token")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(stdout.String())
}

// gitCredentialToken asks the configured git credential helper for the
// github.com entry; its password is the stored OAuth token. Helper prompts
// are suppressed so a missing credential can neither hang nor pop up UI.
func gitCredentialToken() string {
	ctx, cancel := helperContext()
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "credential", "fill")
	cmd.Stdin = strings.NewReader("protocol=https\nhost=github.com\n\n")
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=never",
		"GIT_ASKPASS=echo",
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return ""
	}
	return gitCredentialPassword(stdout.String())
}

// gitCredentialPassword picks the password= line from a "git credential" reply.
func gitCredentialPassword(out string) string {
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		if line := scanner.Text(); strings.HasPrefix(line, "password=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "password="))
		}
	}
	return ""
}

func helperContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), externalTokenRunTimeout)
}
