package install

import (
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/inherelab/eget/internal/client"
	pb "github.com/schollz/progressbar/v3"
)

type RateLimit = client.RateLimit

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
		ProxyURL:          opts.ProxyURL,
		APICacheEnabled:   opts.APICacheEnabled,
		APICacheDir:       opts.APICacheDir,
		APICacheTime:      opts.APICacheTime,
		GhproxyEnabled:    opts.GhproxyEnabled,
		GhproxyHostURL:    opts.GhproxyHostURL,
		GhproxySupportAPI: opts.GhproxySupportAPI,
		GhproxyFallbacks:  append([]string(nil), opts.GhproxyFallbacks...),
		DisableSSL:        opts.DisableSSL,
	}
}

func SetVerbose(enabled bool, writer io.Writer) {
	verboseEnabled = enabled
	verboseWriter = writer
	client.SetVerbose(enabled, writer)
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

func Download(url string, out io.Writer, getbar func(size int64) *pb.ProgressBar, opts Options) error {
	restoreDownloadGet := client.SetDownloadGetWithOptionsForTest(func(url string, clientOpts client.Options) (*http.Response, error) {
		return downloadGetWithOptions(url, opts)
	})
	defer restoreDownloadGet()
	restoreProxyNotice := client.SetProxyNoticeWriter(proxyNoticeWriter)
	defer client.SetProxyNoticeWriter(restoreProxyNotice)
	client.SetVerbose(verboseEnabled, verboseWriter)
	return client.Download(url, out, getbar, ClientOptions(opts))
}

func verbosef(format string, args ...any) {
	client.Verbosef(format, args...)
}

func CacheFilePath(cacheDir, url string) string {
	return client.CacheFilePath(cacheDir, url)
}

func APICacheFilePath(cacheDir, rawURL string) string {
	return client.APICacheFilePath(cacheDir, rawURL)
}

func proxyFuncFor(proxyURL string) (func(*http.Request) (*url.URL, error), error) {
	return client.ProxyFuncFor(proxyURL)
}
