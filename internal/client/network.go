package client

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/inherelab/eget/internal/cachemirror"
)

type Options struct {
	ProxyURL         string
	ProxyExclude     []string
	APICacheEnabled  bool
	APICacheDir      string
	APICacheTime     int
	GhproxyEnabled   bool
	GhproxyHostURL   string
	GhproxyFallbacks []string
	DisableSSL       bool
	Retries          int
	ChunkConcurrency int
	UserAgent        string
	CacheMirror      cachemirror.Options
}

type DownloadResult struct {
	LastModified string
	Filename     string
}

type HTTPGetterFunc func(url string) (*http.Response, error)

func (f HTTPGetterFunc) Get(url string) (*http.Response, error) {
	return f(url)
}

func Get(rawURL string, disableSSL bool) (*http.Response, error) {
	return GetWithOptions(rawURL, Options{DisableSSL: disableSSL})
}

func GetWithOptions(rawURL string, opts Options) (*http.Response, error) {
	client, err := newHTTPClient(opts)
	if err != nil {
		return nil, err
	}

	originalURL, err := urlpkgParse(rawURL)
	if err != nil {
		return nil, err
	}

	cachePath, useAPICache := resolvedAPICachePath(opts, rawURL, originalURL)
	if useAPICache {
		if resp, ok, err := loadAPICacheResponse(cachePath, opts.APICacheTime); err != nil {
			verbosef("api cache read error: %v", err)
		} else if ok {
			verbosef("api cache hit: %s", cachePath)
			printAPICacheNotice(cachePath)
			return resp, nil
		} else {
			verbosef("api cache miss: %s", cachePath)
		}
	}
	if useAPICache && opts.CacheMirror.Active() {
		hit, err := tryAPICacheMirror(cachePath, opts.CacheMirror)
		if err != nil {
			if !opts.CacheMirror.Fallback {
				return nil, err
			}
			verbosef("cache mirror metadata fallback: %v", err)
		}
		if hit {
			resp, ok, err := loadAPICacheResponse(cachePath, 0)
			if err != nil {
				return nil, fmt.Errorf("load mirrored api cache: %w", err)
			}
			if !ok {
				return nil, fmt.Errorf("mirrored api cache is unavailable: %s", cachePath)
			}
			return resp, nil
		}
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
			req, err := http.NewRequest(http.MethodGet, attemptURL, nil)
			if err != nil {
				return nil, err
			}
			if err := setAuthHeader(req, opts.DisableSSL); err != nil {
				return nil, err
			}
			setDefaultHeaders(req, opts)
			printDownloadProxyNoticeForRequest(rawURL, req.URL, opts)
			if isGitHubAPIRequest(originalURL) && shouldUseConfiguredProxyURL(req.URL, opts.ProxyURL, opts.ProxyExclude) {
				printProxyNotice("GitHub API request", opts.ProxyURL)
			}
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
		if useAPICache && resp.StatusCode == http.StatusOK {
			cachedResp, err := storeAPICacheResponse(cachePath, resp)
			if err != nil {
				verbosef("api cache write error: %v", err)
				return resp, nil
			}
			verbosef("api cache store: %s", cachePath)
			return cachedResp, nil
		}

		return resp, nil
	}

	return nil, lastErr
}

func NewHTTPGetter(opts Options) HTTPGetterFunc {
	return HTTPGetterFunc(func(rawURL string) (*http.Response, error) {
		return GetWithOptions(rawURL, opts)
	})
}

type RateLimitJSON struct {
	Resources map[string]RateLimit `json:"resources"`
}

type RateLimit struct {
	Limit     int   `json:"limit"`
	Remaining int   `json:"remaining"`
	Reset     int64 `json:"reset"`
}

func (r RateLimit) ResetTime() time.Time {
	return time.Unix(r.Reset, 0)
}

func GetRateLimit(opts Options) (RateLimit, error) {
	resp, err := GetWithOptions("https://api.github.com/rate_limit", opts)
	if err != nil {
		return RateLimit{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return RateLimit{}, err
	}

	var parsed RateLimitJSON
	if err := json.Unmarshal(body, &parsed); err != nil {
		return RateLimit{}, err
	}
	return parsed.Resources["core"], nil
}

func urlpkgParse(rawURL string) (*url.URL, error) {
	return url.Parse(rawURL)
}

func isLocalFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func responseFilename(resp *http.Response, rawURL string) string {
	if resp != nil {
		if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil {
			if name := params["filename"]; name != "" {
				return path.Base(name)
			}
		}
	}
	return urlPathFilename(rawURL)
}

func urlPathFilename(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err == nil && u.Path != "" {
		return path.Base(u.Path)
	}
	return path.Base(rawURL)
}
