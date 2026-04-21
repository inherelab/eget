package install

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/util"
	pb "github.com/schollz/progressbar/v3"
)

var downloadGet = Get
var downloadGetWithOptions = GetWithOptions
var httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
	return client.Do(req)
}
var proxyNoticeWriter io.Writer = os.Stderr
var apiCacheNoticeWriter io.Writer = os.Stderr
var verboseWriter io.Writer = os.Stderr
var verboseEnabled bool

func SetVerbose(enabled bool, writer io.Writer) {
	verboseEnabled = enabled
	if writer == nil {
		verboseWriter = io.Discard
		return
	}
	verboseWriter = writer
}

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

func getGitHubToken() (string, error) {
	if os.Getenv("EGET_GITHUB_TOKEN") != "" {
		return tokenFrom(os.Getenv("EGET_GITHUB_TOKEN"))
	}
	if os.Getenv("GITHUB_TOKEN") != "" {
		return tokenFrom(os.Getenv("GITHUB_TOKEN"))
	}
	return "", ErrNoToken
}

func setAuthHeader(req *http.Request, disableSSL bool) error {
	token, err := getGitHubToken()
	if err != nil {
		if errors.Is(err, ErrNoToken) {
			return nil
		}
		fmt.Fprintln(os.Stderr, "warning: not using github token:", err)
		return nil
	}

	if req.URL.Scheme == "https" && req.Host == "api.github.com" {
		if disableSSL {
			return fmt.Errorf("cannot use GitHub token if SSL verification is disabled")
		}
		req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	}
	return nil
}

func Get(url string, disableSSL bool) (*http.Response, error) {
	return GetWithOptions(url, Options{DisableSSL: disableSSL})
}

func GetWithOptions(url string, opts Options) (*http.Response, error) {
	client, err := newHTTPClient(opts)
	if err != nil {
		return nil, err
	}

	originalURL, err := urlpkgParse(url)
	if err != nil {
		return nil, err
	}

	if isGitHubAPIRequest(originalURL) {
		printProxyNotice("GitHub API request", opts.ProxyURL)
	}

	cachePath, useAPICache := resolvedAPICachePath(opts, url, originalURL)
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

	attempts := requestAttemptURLs(url, originalURL, opts)
	var lastErr error
	for i, attemptURL := range attempts {
		req, err := http.NewRequest("GET", attemptURL, nil)
		if err != nil {
			return nil, err
		}
		if err := setAuthHeader(req, opts.DisableSSL); err != nil {
			return nil, err
		}

		if attemptURL != url {
			verbosef("ghproxy rewrite: %s -> %s", url, attemptURL)
		}
		if len(attempts) > 1 {
			verbosef("ghproxy attempt %d/%d: %s", i+1, len(attempts), attemptURL)
		}

		verbosef("request: %s %s", req.Method, req.URL.String())
		resp, err := httpDo(client, req)
		if err != nil {
			verbosef("request error: %v", err)
			lastErr = err
			if i < len(attempts)-1 {
				verbosef("ghproxy fallback: switching to next host")
				continue
			}
			return nil, err
		}
		verbosef("response: %s %s", req.URL.String(), resp.Status)

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
	return HTTPGetterFunc(func(url string) (*http.Response, error) {
		return GetWithOptions(url, opts)
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

func newHTTPClient(opts Options) (*http.Client, error) {
	proxyFunc, err := proxyFuncFor(opts.ProxyURL)
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: &http.Transport{
		Proxy:           proxyFunc,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: opts.DisableSSL},
	}}, nil
}

func proxyFuncFor(proxyURL string) (func(*http.Request) (*url.URL, error), error) {
	if proxyURL == "" {
		return http.ProxyFromEnvironment, nil
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy_url %q: %w", proxyURL, err)
	}
	return http.ProxyURL(parsed), nil
}

func Download(url string, out io.Writer, getbar func(size int64) *pb.ProgressBar, opts Options) error {
	if IsLocalFile(url) {
		file, err := os.Open(url)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(out, file)
		return err
	}

	printProxyNotice("download request", opts.ProxyURL)

	resp, err := downloadGetWithOptions(url, opts)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	verbosef("download response bytes: %d", resp.ContentLength)

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("download error: %d: %s", resp.StatusCode, body)
	}

	bar := getbar(resp.ContentLength)
	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	return err
}

func printProxyNotice(kind, proxyURL string) {
	if proxyURL == "" || proxyNoticeWriter == nil {
		return
	}
	ccolor.Fprintf(proxyNoticeWriter, " - Using <ylw>proxy_url for %s</>: %s\n", kind, proxyURL)
}

func printAPICacheNotice(cachePath string) {
	if cachePath == "" || apiCacheNoticeWriter == nil {
		return
	}
	ccolor.Fprintf(apiCacheNoticeWriter, " - Using <ylw>api_cache file</>: %s\n", filepath.Base(cachePath))
}

func verbosef(format string, args ...any) {
	if !verboseEnabled || verboseWriter == nil {
		return
	}
	ccolor.Fprintf(verboseWriter, "<ylw>verbose</> "+format+"\n", args...)
}

func VerboseEnabledForTest() bool {
	return verboseEnabled
}

func isGitHubAPIRequest(u *url.URL) bool {
	return u != nil && strings.EqualFold(u.Host, "api.github.com")
}

func CacheFilePath(cacheDir, url string) string {
	if cacheDir == "" || url == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(url))
	ext := filepath.Ext(url)
	if ext == "" {
		ext = ".bin"
	}
	return filepath.Join(cacheDir, hex.EncodeToString(sum[:])+ext)
}

func APICacheFilePath(cacheDir, rawURL string) string {
	if cacheDir == "" || rawURL == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(rawURL))
	return filepath.Join(cacheDir, hex.EncodeToString(sum[:])+".json")
}

func requestAttemptURLs(rawURL string, parsed *url.URL, opts Options) []string {
	if !opts.GhproxyEnabled {
		return []string{rawURL}
	}
	if parsed == nil {
		return []string{rawURL}
	}
	if !isGitHubDownloadRequest(parsed) && !(opts.GhproxySupportAPI && isGitHubAPIRequest(parsed)) {
		return []string{rawURL}
	}

	hosts := make([]string, 0, 1+len(opts.GhproxyFallbacks))
	if opts.GhproxyHostURL != "" {
		hosts = append(hosts, opts.GhproxyHostURL)
	}
	hosts = append(hosts, opts.GhproxyFallbacks...)
	if len(hosts) == 0 {
		return []string{rawURL}
	}

	attempts := make([]string, 0, len(hosts))
	seen := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		host = strings.TrimRight(strings.TrimSpace(host), "/")
		if host == "" {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		attempts = append(attempts, host+"/"+rawURL)
	}
	if len(attempts) == 0 {
		return []string{rawURL}
	}
	return attempts
}

func resolvedAPICachePath(opts Options, rawURL string, parsed *url.URL) (string, bool) {
	if !opts.APICacheEnabled || !isGitHubAPIRequest(parsed) {
		return "", false
	}
	cacheDir := opts.APICacheDir
	if cacheDir == "" {
		return "", false
	}
	expanded, err := util.Expand(cacheDir)
	if err != nil {
		verbosef("api cache expand error: %v", err)
		return "", false
	}
	return APICacheFilePath(expanded, rawURL), true
}

func loadAPICacheResponse(path string, cacheTime int) (*http.Response, bool, error) {
	if path == "" {
		return nil, false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if cacheTime > 0 && time.Since(info.ModTime()) > time.Duration(cacheTime)*time.Second {
		verbosef("api cache expired: %s", path)
		return nil, false, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	return &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Body:          io.NopCloser(strings.NewReader(string(body))),
		ContentLength: int64(len(body)),
		Header:        make(http.Header),
	}, true, nil
}

func storeAPICacheResponse(path string, resp *http.Response) (*http.Response, error) {
	if path == "" || resp == nil || resp.Body == nil {
		return resp, nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(strings.NewReader(string(body)))
	resp.ContentLength = int64(len(body))
	return resp, nil
}

func isGitHubDownloadRequest(u *url.URL) bool {
	return u != nil && strings.EqualFold(u.Host, "github.com")
}

func urlpkgParse(rawURL string) (*url.URL, error) {
	return url.Parse(rawURL)
}
