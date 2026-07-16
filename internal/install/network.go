package install

import (
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/inherelab/eget/internal/client"
)

type RateLimit = client.RateLimit
type CacheMeta = client.CacheMeta
type DownloadResult = client.DownloadResult

var downloadGetWithOptions = GetWithOptions
var httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
	return client.Do(req)
}
var proxyNoticeWriter io.Writer = os.Stderr
var apiCacheNoticeWriter io.Writer = os.Stderr
var verboseWriter io.Writer = os.Stderr
var verboseEnabled bool

func ClientOptions(opts Options) client.Options {
	return client.Options{
		ProxyURL:         opts.ProxyURL,
		ProxyExclude:     append([]string(nil), opts.ProxyExclude...),
		APICacheEnabled:  opts.APICacheEnabled,
		APICacheDir:      opts.APICacheDir,
		APICacheTime:     opts.APICacheTime,
		GhproxyEnabled:   opts.GhproxyEnabled,
		GhproxyHostURL:   opts.GhproxyHostURL,
		GhproxyFallbacks: append([]string(nil), opts.GhproxyFallbacks...),
		DisableSSL:       opts.DisableSSL,
		ChunkConcurrency: opts.ChunkConcurrency,
		UserAgent:        opts.UserAgent,
		CacheMirror:      opts.CacheMirror,
	}
}

func SetVerbose(enabled bool, writer io.Writer) {
	verboseEnabled = enabled
	verboseWriter = writer
	client.SetVerbose(enabled, writer)
}

func SetProxyNoticeWriter(writer io.Writer) io.Writer {
	prev := proxyNoticeWriter
	proxyNoticeWriter = writer
	return prev
}

func SetAPICacheNoticeWriter(writer io.Writer) io.Writer {
	prev := apiCacheNoticeWriter
	apiCacheNoticeWriter = writer
	return prev
}

func VerboseEnabledForTest() bool {
	return client.VerboseEnabledForTest()
}

func Get(url string, disableSSL bool) (*http.Response, error) {
	return client.Get(url, disableSSL)
}

func GetWithOptions(url string, opts Options) (*http.Response, error) {
	restoreHTTPDo := client.SetHTTPDoForTest(httpDo)
	defer restoreHTTPDo()
	restoreProxyNotice := client.SetProxyNoticeWriter(proxyNoticeWriter)
	defer client.SetProxyNoticeWriter(restoreProxyNotice)
	restoreAPICacheNotice := client.SetAPICacheNoticeWriter(apiCacheNoticeWriter)
	defer client.SetAPICacheNoticeWriter(restoreAPICacheNotice)
	client.SetVerbose(verboseEnabled, verboseWriter)
	return client.GetWithOptions(url, ClientOptions(opts))
}

func NewHTTPGetter(opts Options) HTTPGetterFunc {
	return HTTPGetterFunc(func(url string) (*http.Response, error) {
		return GetWithOptions(url, opts)
	})
}

func GetRateLimit(opts Options) (RateLimit, error) {
	return client.GetRateLimit(ClientOptions(opts))
}

func Download(url string, out io.Writer, getbar func(size int64) io.Writer, opts Options) error {
	_, err := DownloadWithResult(url, out, getbar, opts)
	return err
}

func DownloadWithResult(url string, out io.Writer, getbar func(size int64) io.Writer, opts Options) (DownloadResult, error) {
	restoreDownloadGet := client.SetDownloadGetWithOptionsForTest(func(url string, clientOpts client.Options) (*http.Response, error) {
		return downloadGetWithOptions(url, opts)
	})
	defer restoreDownloadGet()
	restoreProxyNotice := client.SetProxyNoticeWriter(proxyNoticeWriter)
	defer client.SetProxyNoticeWriter(restoreProxyNotice)
	client.SetVerbose(verboseEnabled, verboseWriter)
	return client.DownloadWithResult(url, out, getbar, ClientOptions(opts))
}

func DownloadFile(url, target string, getbar func(size int64) io.Writer, opts Options) (client.DownloadFileResult, error) {
	restoreDownloadGet := client.SetDownloadGetWithOptionsForTest(func(url string, clientOpts client.Options) (*http.Response, error) {
		return downloadGetWithOptions(url, opts)
	})
	defer restoreDownloadGet()
	restoreHTTPDo := client.SetHTTPDoForTest(httpDo)
	defer restoreHTTPDo()
	restoreProxyNotice := client.SetProxyNoticeWriter(proxyNoticeWriter)
	defer client.SetProxyNoticeWriter(restoreProxyNotice)
	client.SetVerbose(verboseEnabled, verboseWriter)
	return client.DownloadFile(url, target, getbar, ClientOptions(opts))
}

func ProbeLastModified(url string, opts Options) string {
	restoreHTTPDo := client.SetHTTPDoForTest(httpDo)
	defer restoreHTTPDo()
	restoreProxyNotice := client.SetProxyNoticeWriter(proxyNoticeWriter)
	defer client.SetProxyNoticeWriter(restoreProxyNotice)
	client.SetVerbose(verboseEnabled, verboseWriter)
	return client.ProbeLastModified(url, ClientOptions(opts))
}

func verbosef(format string, args ...any) {
	client.Verbosef(format, args...)
}

func CacheFilePath(cacheDir, url string) string {
	return client.CacheFilePath(cacheDir, url)
}

func CacheFilePathWithMeta(cacheDir, url string, meta CacheMeta) string {
	return client.CacheFilePathWithMeta(cacheDir, url, meta)
}

func APICacheFilePath(cacheDir, rawURL string) string {
	return client.APICacheFilePath(cacheDir, rawURL)
}

func proxyFuncFor(proxyURL string, exclude []string) (func(*http.Request) (*url.URL, error), error) {
	return client.ProxyFuncFor(proxyURL, exclude)
}
