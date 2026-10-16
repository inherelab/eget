package client

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/inherelab/eget/internal/util"
)

func tokenFrom(value string) (string, error) {
	if strings.HasPrefix(value, "@") {
		file, err := util.Expand(value[1:])
		if err != nil {
			return "", err
		}
		body, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(body), "\r\n"), nil
	}
	return value, nil
}

var ErrNoToken = errors.New("no github token")

func getGitHubToken(external string) (string, error) {
	if external != "" {
		return tokenFrom(external)
	}
	if os.Getenv("EGET_GITHUB_TOKEN") != "" {
		return tokenFrom(os.Getenv("EGET_GITHUB_TOKEN"))
	}
	if os.Getenv("GITHUB_TOKEN") != "" {
		return tokenFrom(os.Getenv("GITHUB_TOKEN"))
	}
	return externalGitHubToken()
}

func setAuthHeader(req *http.Request, opts Options) error {
	token, err := getGitHubToken(opts.GithubToken)
	if err != nil {
		if errors.Is(err, ErrNoToken) {
			return nil
		}
		fmt.Fprintln(os.Stderr, "warning: not using github token:", err)
		return nil
	}

	if req.URL.Scheme == "https" && req.Host == "api.github.com" {
		if opts.DisableSSL {
			return fmt.Errorf("cannot use GitHub token if SSL verification is disabled")
		}
		req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	}
	return nil
}

func setDefaultHeaders(req *http.Request, opts Options) {
	if req == nil || req.URL == nil {
		return
	}
	userAgent := opts.UserAgent
	if userAgent == "" {
		if !isSourceForgeDownloadRequest(req.URL) {
			userAgent = DefaultUserAgent
		}
	}
	if userAgent != "" && req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", userAgent)
	}
	if strings.EqualFold(req.URL.Hostname(), "sourceforge.net") && req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	}
}
