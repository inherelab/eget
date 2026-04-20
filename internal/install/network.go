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

	"github.com/inherelab/eget/internal/util"
	pb "github.com/schollz/progressbar/v3"
)

var downloadGet = Get
var downloadGetWithOptions = GetWithOptions
var httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
	return client.Do(req)
}
var proxyNoticeWriter io.Writer = os.Stderr

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
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if err := setAuthHeader(req, opts.DisableSSL); err != nil {
		return nil, err
	}

	client, err := newHTTPClient(opts)
	if err != nil {
		return nil, err
	}

	if isGitHubAPIRequest(req.URL) {
		printProxyNotice("GitHub API request", opts.ProxyURL)
	}

	return httpDo(client, req)
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
	req, err := http.NewRequest("GET", "https://api.github.com/rate_limit", nil)
	if err != nil {
		return RateLimit{}, err
	}
	if err := setAuthHeader(req, opts.DisableSSL); err != nil {
		return RateLimit{}, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client, err := newHTTPClient(opts)
	if err != nil {
		return RateLimit{}, err
	}

	resp, err := httpDo(client, req)
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
	fmt.Fprintf(proxyNoticeWriter, "Using proxy_url for %s: %s\n", kind, proxyURL)
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
