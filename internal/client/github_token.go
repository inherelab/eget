package client

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// externalTokenRunTimeout bounds each helper invocation so a misbehaving
// helper cannot stall requests.
const externalTokenRunTimeout = 5 * time.Second

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
func externalGitHubToken() (string, error) {
	token, err := resolveExternalGitHubToken()
	if err != nil || token == "" {
		return "", ErrNoToken
	}
	return token, nil
}

var (
	resolveExternalGitHubToken = resolveExternalGitHubTokenOnce
	externalResolveOnce        sync.Once
	externalResolvedToken      string
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
