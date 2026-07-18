package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"

	"github.com/inherelab/eget/internal/config"
	"golang.org/x/net/http/httpproxy"
)

func ProbeLastModified(rawURL string, opts Options) string {
	resp, err := requestWithOptions(http.MethodHead, rawURL, "", opts)
	if err != nil {
		verbosef("last-modified probe failed: %v", err)
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		verbosef("last-modified probe unsupported status: %s", resp.Status)
		return ""
	}
	return resp.Header.Get("Last-Modified")
}

func newHTTPClient(opts Options) (*http.Client, error) {
	proxyFunc, err := ProxyFuncFor(opts.ProxyURL, opts.ProxyExclude)
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: &http.Transport{
		Proxy:           proxyFunc,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: opts.DisableSSL},
	}}, nil
}

func ProxyFuncFor(proxyURL string, exclude []string) (func(*http.Request) (*url.URL, error), error) {
	if proxyURL == "" {
		proxyFunc := httpproxy.FromEnvironment().ProxyFunc()
		return func(req *http.Request) (*url.URL, error) {
			return proxyFunc(req.URL)
		}, nil
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy_url %q: %w", proxyURL, err)
	}
	return func(req *http.Request) (*url.URL, error) {
		if config.ProxyExcluded(req.URL.Host, exclude) {
			return nil, nil
		}
		return parsed, nil
	}, nil
}

func requestWithOptions(method, rawURL, rangeHeader string, opts Options) (*http.Response, error) {
	client, err := newHTTPClient(opts)
	if err != nil {
		return nil, err
	}
	originalURL, err := urlpkgParse(rawURL)
	if err != nil {
		return nil, err
	}
	attempts := requestAttemptURLs(rawURL, originalURL, opts)
	retries := requestRetries(opts, isProviderMetadataRequest(originalURL))
	var lastErr error
	for i, attemptURL := range attempts {
		if attemptURL != rawURL {
			verbosef("ghproxy rewrite: %s -> %s", rawURL, attemptURL)
		}
		if len(attempts) > 1 {
			verbosef("ghproxy attempt %d/%d: %s", i+1, len(attempts), attemptURL)
		}
		resp, err := doRequestWithRetries(client, retries, func() (*http.Request, error) {
			req, err := http.NewRequest(method, attemptURL, nil)
			if err != nil {
				return nil, err
			}
			if rangeHeader != "" {
				req.Header.Set("Range", rangeHeader)
			}
			if err := setAuthHeader(req, opts.DisableSSL); err != nil {
				return nil, err
			}
			setDefaultHeaders(req, opts)
			printDownloadProxyNoticeForRequest(rawURL, req.URL, opts)
			return req, nil
		})
		if err != nil {
			lastErr = err
			if i < len(attempts)-1 {
				verbosef("ghproxy fallback: switching to next host")
				continue
			}
			return nil, err
		}
		return resp, nil
	}
	return nil, lastErr
}

func requestRetries(opts Options, providerMetadata bool) int {
	if providerMetadata || opts.Retries < 1 {
		return 1
	}
	return opts.Retries
}

func doRequestWithRetries(client *http.Client, retries int, newRequest func() (*http.Request, error)) (*http.Response, error) {
	var lastErr error
	for attempt := 1; attempt <= retries; attempt++ {
		req, err := newRequest()
		if err != nil {
			return nil, err
		}
		verbosef("request: %s %s", req.Method, req.URL.String())
		resp, err := httpDo(client, req)
		if err == nil {
			verbosef("response: %s %s", req.URL.String(), resp.Status)
			return resp, nil
		}
		lastErr = err
		verbosef("request error: %v", err)
		if attempt < retries {
			verbosef("request retry %d/%d: %s", attempt+1, retries, req.URL.String())
		}
	}
	return nil, lastErr
}
