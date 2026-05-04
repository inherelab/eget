package install

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/ccolor"
	sourcegithub "github.com/inherelab/eget/internal/source/github"
	sourcesf "github.com/inherelab/eget/internal/source/sourceforge"
)

func TestCacheFilePath(t *testing.T) {
	cacheDir := t.TempDir()
	got := CacheFilePath(cacheDir, "https://example.com/tool.tar.gz")
	if filepath.Dir(got) != cacheDir {
		t.Fatalf("expected cache file under %q, got %q", cacheDir, got)
	}
	if filepath.Ext(got) != ".gz" {
		t.Fatalf("expected extension .gz, got %q", filepath.Ext(got))
	}
}

func TestDownloadBodyUsesCacheWhenAvailable(t *testing.T) {
	cacheDir := t.TempDir()
	url := "https://example.com/tool.tar.gz"
	cachePath := CacheFilePath(cacheDir, url)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("cached-data"), 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	calls := 0
	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("network-data")),
		}, nil
	}

	var stderr bytes.Buffer
	runner := &InstallRunner{Stderr: &stderr}
	body, err := runner.downloadBody(url, Options{CacheDir: cacheDir})
	if err != nil {
		t.Fatalf("download body: %v", err)
	}

	if string(body) != "cached-data" {
		t.Fatalf("expected cached data, got %q", string(body))
	}
	if calls != 0 {
		t.Fatalf("expected no network calls, got %d", calls)
	}
	if got := stderr.String(); !strings.Contains(got, "Using cached file") {
		t.Fatalf("expected cached-file notice, got %q", got)
	}
}

func TestDownloadBodyWritesCacheAfterDownload(t *testing.T) {
	cacheDir := t.TempDir()
	url := "https://example.com/tool.tar.gz"
	cachePath := CacheFilePath(cacheDir, url)

	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("network-data")),
		}, nil
	}

	runner := &InstallRunner{Stderr: io.Discard}
	body, err := runner.downloadBody(url, Options{CacheDir: cacheDir})
	if err != nil {
		t.Fatalf("download body: %v", err)
	}
	if string(body) != "network-data" {
		t.Fatalf("expected network data, got %q", string(body))
	}

	saved, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	if string(saved) != "network-data" {
		t.Fatalf("expected cached network data, got %q", string(saved))
	}
}

func TestDownloadPrintsProxyNoticeForRemoteRequest(t *testing.T) {
	var notice bytes.Buffer
	origNoticeWriter := proxyNoticeWriter
	origGetWithOptions := downloadGetWithOptions
	defer func() {
		proxyNoticeWriter = origNoticeWriter
		downloadGetWithOptions = origGetWithOptions
	}()
	proxyNoticeWriter = &notice
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		if opts.ProxyURL != "http://127.0.0.1:7890" {
			t.Fatalf("expected proxy url to propagate, got %q", opts.ProxyURL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("network-data")),
		}, nil
	}

	err := Download("https://example.com/tool.tar.gz", io.Discard, func(size int64) io.Writer {
		return io.Discard
	}, Options{ProxyURL: "http://127.0.0.1:7890"})
	if err != nil {
		t.Fatalf("Download(): %v", err)
	}

	if got := notice.String(); !strings.Contains(got, "proxy_url for download request") {
		t.Fatalf("expected download proxy notice, got %q", got)
	}
}

type recordingProgress struct {
	bytes    int
	finished bool
}

func (p *recordingProgress) Write(data []byte) (int, error) {
	p.bytes += len(data)
	return len(data), nil
}

func (p *recordingProgress) Finish(...string) {
	p.finished = true
}

func TestDownloadWritesAndFinishesProgressWriter(t *testing.T) {
	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: 12,
			Body:          io.NopCloser(strings.NewReader("network-data")),
		}, nil
	}

	progress := &recordingProgress{}
	var out bytes.Buffer
	err := Download("https://example.com/tool.tar.gz", &out, func(size int64) io.Writer {
		if size != 12 {
			t.Fatalf("expected content length 12, got %d", size)
		}
		return progress
	}, Options{})
	if err != nil {
		t.Fatalf("Download(): %v", err)
	}
	if out.String() != "network-data" {
		t.Fatalf("expected downloaded body, got %q", out.String())
	}
	if progress.bytes != len("network-data") {
		t.Fatalf("expected progress bytes %d, got %d", len("network-data"), progress.bytes)
	}
	if !progress.finished {
		t.Fatal("expected progress writer to be finished")
	}
}

func TestEffectiveOutputUsesGuiTargetForPortableGUI(t *testing.T) {
	opts := Options{Output: "C:/Tools", GuiTarget: "C:/Program/AITools", IsGUI: true, InstallMode: InstallModePortable}
	got := effectiveOutput(opts)
	if got != "C:/Program/AITools" {
		t.Fatalf("expected gui target, got %q", got)
	}
}

func TestEffectiveOutputKeepsExplicitOutputForPortableGUI(t *testing.T) {
	opts := Options{Output: "D:/Custom/PicoClaw", GuiTarget: "C:/Program/AITools", IsGUI: true, InstallMode: InstallModePortable, OutputExplicit: true}
	got := effectiveOutput(opts)
	if got != "D:/Custom/PicoClaw" {
		t.Fatalf("expected explicit output, got %q", got)
	}
}

type fakeInstallerLauncher struct {
	path string
	kind InstallerKind
	err  error
}

func (f *fakeInstallerLauncher) LaunchInstaller(path string, kind InstallerKind) error {
	f.path = path
	f.kind = kind
	return f.err
}

func TestLaunchGUIInstallerReturnsInstallerResult(t *testing.T) {
	launcher := &fakeInstallerLauncher{}
	runner := &InstallRunner{InstallerLauncher: launcher}
	file := ExtractedFile{Name: "PicoClaw-Setup.exe", ArchiveName: "PicoClaw-Setup.exe"}
	path := filepath.Join(t.TempDir(), "PicoClaw-Setup.exe")
	if err := os.WriteFile(path, []byte("installer"), 0o755); err != nil {
		t.Fatalf("write installer: %v", err)
	}
	result, err := runner.launchGUIInstaller(path, file, Options{IsGUI: true})
	if err != nil {
		t.Fatalf("launch gui installer: %v", err)
	}
	if result.InstallMode != InstallModeInstaller || !result.IsGUI || result.InstallerFile != path {
		t.Fatalf("expected installer gui result, got %#v", result)
	}
	if launcher.path != path || launcher.kind != InstallerKindEXE {
		t.Fatalf("unexpected launcher call path=%q kind=%q", launcher.path, launcher.kind)
	}
}

func TestRunPromptsBeforeLaunchingDetectedInstallerWithoutGUIFlag(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "PicoClaw-Setup.exe")
	if err := os.WriteFile(path, []byte("installer"), 0o755); err != nil {
		t.Fatalf("write installer: %v", err)
	}

	launcher := &fakeInstallerLauncher{}
	runner := NewRunner(NewDefaultService(nil, nil))
	runner.InstallerLauncher = launcher
	runner.Stderr = io.Discard
	var prompted string
	runner.ConfirmLaunchInstaller = func(file string) (bool, error) {
		prompted = file
		return true, nil
	}

	result, err := runner.Run(path, Options{})
	if err != nil {
		t.Fatalf("run installer: %v", err)
	}
	if prompted != "PicoClaw-Setup.exe" {
		t.Fatalf("expected setup file prompt, got %q", prompted)
	}
	if launcher.path != path || launcher.kind != InstallerKindEXE {
		t.Fatalf("unexpected launcher call path=%q kind=%q", launcher.path, launcher.kind)
	}
	if !result.IsGUI || result.InstallMode != InstallModeInstaller {
		t.Fatalf("expected confirmed installer result, got %#v", result)
	}
}

func TestRunQuietSuppressesInstallNotice(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "tool.exe")
	if err := os.WriteFile(source, []byte("tool"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}
	outputDir := filepath.Join(tmpDir, "bin")
	if err := os.Mkdir(outputDir, 0o755); err != nil {
		t.Fatalf("mkdir output: %v", err)
	}

	var stderr bytes.Buffer
	var globalOut bytes.Buffer
	ccolor.SetOutput(&globalOut)
	defer ccolor.SetOutput(os.Stdout)

	runner := NewRunner(NewDefaultService(nil, nil))
	runner.Stderr = &stderr

	if _, err := runner.Run(source, Options{Quiet: true, DownloadOnly: true, Output: outputDir}); err != nil {
		t.Fatalf("run quiet install: %v", err)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected quiet stderr to be empty, got %q", got)
	}
	if got := globalOut.String(); got != "" {
		t.Fatalf("expected quiet global output to be empty, got %q", got)
	}
}

func TestRunStopsWhenConfiguredAssetFilterMatchesNoCurrentReleaseAssets(t *testing.T) {
	svc := NewDefaultService(nil, nil)
	svc.GitHubGetterFactory = func(opts Options) sourcegithub.HTTPGetter {
		return HTTPGetterFunc(func(url string) (*http.Response, error) {
			if !strings.Contains(url, "/repos/Zxilly/go-size-analyzer/releases/latest") {
				t.Fatalf("unexpected GitHub API request %q", url)
			}
			body := `{"assets":[{"browser_download_url":"https://github.com/Zxilly/go-size-analyzer/releases/download/v1.12.5/go-size-analyzer_1.12.5_linux_amd64.tar.gz"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})
	}

	downloadCalls := 0
	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		downloadCalls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("old-asset")),
		}, nil
	}

	runner := NewRunner(svc)
	runner.Stderr = io.Discard
	runner.InstalledLoad = func() (map[string]string, map[string]string, error) {
		return map[string]string{}, map[string]string{
			"Zxilly/go-size-analyzer": "https://github.com/Zxilly/go-size-analyzer/releases/download/v1.12.4/go-size-analyzer_1.12.4_windows_amd64.zip",
		}, nil
	}

	_, err := runner.Run("Zxilly/go-size-analyzer", Options{
		System: "windows/amd64",
		Asset:  []string{"windows"},
	})
	if err == nil || !strings.Contains(err.Error(), "asset `windows` not found") {
		t.Fatalf("expected missing current asset error, got %v", err)
	}
	if downloadCalls != 0 {
		t.Fatalf("expected no download when current release asset does not match, got %d calls", downloadCalls)
	}
}

func TestRunFallsBackToOlderSourceForgeVersionWhenAssetMissing(t *testing.T) {
	responses := map[string]string{
		"https://sourceforge.net/projects/keepass/files/Translations%202.x/": `
<script>
net.sf.files = {
  "2.59": {"name":"2.59","full_path":"/Translations 2.x/2.59","type":"d"},
  "2.60": {"name":"2.60","full_path":"/Translations 2.x/2.60","type":"d"},
  "2.61": {"name":"2.61","full_path":"/Translations 2.x/2.61","type":"d"}
};
</script>`,
		"https://sourceforge.net/projects/keepass/files/Translations%202.x/2.61/": `
<script>
net.sf.files = {
  "Spanish.zip": {
    "name":"Spanish.zip",
    "download_url":"https://downloads.sourceforge.net/project/keepass/Translations%202.x/2.61/Spanish.zip",
    "full_path":"/Translations 2.x/2.61/Spanish.zip",
    "type":"f"
  }
};
</script>`,
		"https://sourceforge.net/projects/keepass/files/Translations%202.x/2.60/": `
<script>
net.sf.files = {
  "German.zip": {
    "name":"German.zip",
    "download_url":"https://downloads.sourceforge.net/project/keepass/Translations%202.x/2.60/German.zip",
    "full_path":"/Translations 2.x/2.60/German.zip",
    "type":"f"
  }
};
</script>`,
		"https://sourceforge.net/projects/keepass/files/Translations%202.x/2.59/": `
<script>
net.sf.files = {
  "Ukrainian.zip": {
    "name":"Ukrainian.zip",
    "download_url":"https://downloads.sourceforge.net/project/keepass/Translations%202.x/2.59/Ukrainian.zip",
    "full_path":"/Translations 2.x/2.59/Ukrainian.zip",
    "type":"f"
  }
};
</script>`,
	}
	var sourceForgeRequests []string
	svc := NewDefaultService(nil, nil)
	svc.SourceForgeGetterFactory = func(opts Options) sourcesf.HTTPGetter {
		return HTTPGetterFunc(func(url string) (*http.Response, error) {
			sourceForgeRequests = append(sourceForgeRequests, url)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(responses[url])),
			}, nil
		})
	}

	var downloadedURL string
	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		downloadedURL = url
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("translation")),
		}, nil
	}

	outputDir := t.TempDir()
	runner := NewRunner(svc)
	runner.Stderr = io.Discard
	result, err := runner.Run("sourceforge:keepass/Translations 2.x", Options{
		FallbackVersions: 10,
		Asset:            []string{"Ukrainian", "zip"},
		DownloadOnly:     true,
		Output:           outputDir,
	})

	if err != nil {
		t.Fatalf("run sourceforge fallback: %v", err)
	}
	wantURL := "https://downloads.sourceforge.net/project/keepass/Translations%202.x/2.59/Ukrainian.zip"
	if result.URL != wantURL || downloadedURL != wantURL {
		t.Fatalf("expected fallback URL %q, got result=%q downloaded=%q", wantURL, result.URL, downloadedURL)
	}
	assertRequests := []string{
		"https://sourceforge.net/projects/keepass/files/Translations%202.x/",
		"https://sourceforge.net/projects/keepass/files/Translations%202.x/2.61/",
		"https://sourceforge.net/projects/keepass/files/Translations%202.x/",
		"https://sourceforge.net/projects/keepass/files/Translations%202.x/2.60/",
		"https://sourceforge.net/projects/keepass/files/Translations%202.x/2.59/",
	}
	if strings.Join(sourceForgeRequests, "\n") != strings.Join(assertRequests, "\n") {
		t.Fatalf("unexpected sourceforge requests:\n%v", strings.Join(sourceForgeRequests, "\n"))
	}
}

func TestDefaultConfirmLaunchInstallerTreatsBlankLineAsCancel(t *testing.T) {
	origStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	os.Stdin = reader
	defer func() {
		os.Stdin = origStdin
		_ = reader.Close()
	}()
	if _, err := writer.WriteString("\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}

	confirmed, err := defaultConfirmLaunchInstaller("Clash.Verge_2.4.7_x64-setup.exe")
	if err != nil {
		t.Fatalf("expected blank line to cancel without error, got %v", err)
	}
	if confirmed {
		t.Fatal("expected blank line to cancel installer launch")
	}
}

func TestDownloadSkipsProxyNoticeForLocalFile(t *testing.T) {
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "tool.tar.gz")
	if err := os.WriteFile(localFile, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	var notice bytes.Buffer
	origNoticeWriter := proxyNoticeWriter
	defer func() { proxyNoticeWriter = origNoticeWriter }()
	proxyNoticeWriter = &notice

	err := Download(localFile, io.Discard, func(size int64) io.Writer {
		return io.Discard
	}, Options{ProxyURL: "http://127.0.0.1:7890"})
	if err != nil {
		t.Fatalf("Download(local): %v", err)
	}

	if got := notice.String(); got != "" {
		t.Fatalf("expected no proxy notice for local file, got %q", got)
	}
}

func TestNewHTTPGetterUsesProxyURL(t *testing.T) {
	proxyFunc, err := proxyFuncFor("http://127.0.0.1:7890")
	if err != nil {
		t.Fatalf("proxyFuncFor: %v", err)
	}
	req, err := http.NewRequest(http.MethodGet, "https://example.com/tool.tar.gz", nil)
	if err == nil {
		proxyURL, err := proxyFunc(req)
		if err != nil {
			t.Fatalf("proxy func: %v", err)
		}
		if proxyURL == nil {
			t.Fatal("expected proxy url to be returned")
		}
		if proxyURL.String() != "http://127.0.0.1:7890" {
			t.Fatalf("expected proxy url http://127.0.0.1:7890, got %q", proxyURL.String())
		}
		return
	}
	t.Fatalf("new request: %v", err)
}

func TestProxyFuncForRejectsInvalidProxyURL(t *testing.T) {
	_, err := proxyFuncFor("://bad-proxy")
	if err == nil {
		t.Fatal("expected invalid proxy url error")
	}
	if !strings.Contains(err.Error(), "invalid proxy_url") {
		t.Fatalf("expected invalid proxy_url error, got %v", err)
	}
}

func TestProxyFuncForFallsBackToEnvironment(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:7891")
	proxyFunc, err := proxyFuncFor("")
	if err != nil {
		t.Fatalf("proxyFuncFor env fallback: %v", err)
	}
	req := &http.Request{URL: &url.URL{Scheme: "https", Host: "example.com"}}
	proxyURL, err := proxyFunc(req)
	if err != nil {
		t.Fatalf("proxy func env fallback: %v", err)
	}
	if proxyURL == nil {
		t.Fatal("expected environment proxy url to be returned")
	}
	if proxyURL.String() != "http://127.0.0.1:7891" {
		t.Fatalf("expected env proxy url http://127.0.0.1:7891, got %q", proxyURL.String())
	}
}

func TestGetWithOptionsPrintsProxyNoticeForGitHubAPI(t *testing.T) {
	var notice bytes.Buffer
	origNoticeWriter := proxyNoticeWriter
	origHTTPDo := httpDo
	defer func() {
		proxyNoticeWriter = origNoticeWriter
		httpDo = origHTTPDo
	}()
	proxyNoticeWriter = &notice
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	}

	resp, err := GetWithOptions("https://api.github.com/repos/gookit/gitw/releases/latest", Options{ProxyURL: "http://127.0.0.1:7890"})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	_ = resp.Body.Close()

	if got := notice.String(); !strings.Contains(got, "proxy_url for GitHub API request") {
		t.Fatalf("expected GitHub API proxy notice, got %q", got)
	}
}

func TestGetWithOptionsSkipsProxyNoticeWithoutProxyURL(t *testing.T) {
	var notice bytes.Buffer
	origNoticeWriter := proxyNoticeWriter
	origHTTPDo := httpDo
	defer func() {
		proxyNoticeWriter = origNoticeWriter
		httpDo = origHTTPDo
	}()
	proxyNoticeWriter = &notice
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	}

	resp, err := GetWithOptions("https://api.github.com/repos/gookit/gitw/releases/latest", Options{})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	_ = resp.Body.Close()

	if got := notice.String(); got != "" {
		t.Fatalf("expected no proxy notice without proxy_url, got %q", got)
	}
}

func TestGetWithOptionsPrintsVerboseRequestAndResponse(t *testing.T) {
	var verbose bytes.Buffer
	origVerboseEnabled := verboseEnabled
	origVerboseWriter := verboseWriter
	origHTTPDo := httpDo
	defer func() {
		verboseEnabled = origVerboseEnabled
		verboseWriter = origVerboseWriter
		httpDo = origHTTPDo
	}()
	SetVerbose(true, &verbose)
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	}

	resp, err := GetWithOptions("https://api.github.com/repos/gookit/gitw/releases/latest", Options{})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	_ = resp.Body.Close()

	got := verbose.String()
	if !strings.Contains(got, "request: GET https://api.github.com/repos/gookit/gitw/releases/latest") {
		t.Fatalf("expected verbose request log, got %q", got)
	}
	if !strings.Contains(got, "response: https://api.github.com/repos/gookit/gitw/releases/latest 200 OK") {
		t.Fatalf("expected verbose response log, got %q", got)
	}
}

func TestGetWithOptionsUsesAPICacheWhenAvailable(t *testing.T) {
	cacheDir := t.TempDir()
	apiURL := "https://api.github.com/repos/gookit/gitw/releases/latest"
	cachePath := APICacheFilePath(cacheDir, apiURL)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte(`{"tag_name":"v0.3.6"}`), 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	calls := 0
	origHTTPDo := httpDo
	origNoticeWriter := apiCacheNoticeWriter
	defer func() { httpDo = origHTTPDo }()
	defer func() { apiCacheNoticeWriter = origNoticeWriter }()
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"tag_name":"network"}`)),
		}, nil
	}
	var notice bytes.Buffer
	apiCacheNoticeWriter = &notice

	resp, err := GetWithOptions(apiURL, Options{
		APICacheEnabled: true,
		APICacheDir:     cacheDir,
		APICacheTime:    300,
	})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != `{"tag_name":"v0.3.6"}` {
		t.Fatalf("expected cached response body, got %q", string(body))
	}
	if calls != 0 {
		t.Fatalf("expected no network calls, got %d", calls)
	}
	if got := notice.String(); !strings.Contains(got, "api_cache file") {
		t.Fatalf("expected api cache notice, got %q", got)
	}
}

func TestGetWithOptionsWritesAPICacheAfterNetworkRequest(t *testing.T) {
	cacheDir := t.TempDir()
	apiURL := "https://api.github.com/repos/gookit/gitw/releases/latest"

	origHTTPDo := httpDo
	defer func() { httpDo = origHTTPDo }()
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v0.3.6"}`)),
		}, nil
	}

	resp, err := GetWithOptions(apiURL, Options{
		APICacheEnabled: true,
		APICacheDir:     cacheDir,
		APICacheTime:    300,
	})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	cachePath := APICacheFilePath(cacheDir, apiURL)
	saved, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	if string(saved) != `{"tag_name":"v0.3.6"}` {
		t.Fatalf("expected cached response body, got %q", string(saved))
	}
}

func TestGetWithOptionsUsesGhproxyForDownloads(t *testing.T) {
	origHTTPDo := httpDo
	defer func() { httpDo = origHTTPDo }()

	var requested string
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		requested = req.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	}

	resp, err := GetWithOptions("https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe", Options{
		GhproxyEnabled: true,
		GhproxyHostURL: "https://gh.felicity.ac.cn",
	})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	_ = resp.Body.Close()

	want := "https://gh.felicity.ac.cn/https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe"
	if requested != want {
		t.Fatalf("expected ghproxy rewritten url %q, got %q", want, requested)
	}
}

func TestGetWithOptionsUsesGhproxyForGitHubAPIWhenSupported(t *testing.T) {
	origHTTPDo := httpDo
	defer func() { httpDo = origHTTPDo }()

	var requested string
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		requested = req.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	}

	resp, err := GetWithOptions("https://api.github.com/repos/gookit/gitw/releases/latest", Options{
		GhproxyEnabled:    true,
		GhproxyHostURL:    "https://gh.felicity.ac.cn",
		GhproxySupportAPI: true,
	})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	_ = resp.Body.Close()

	want := "https://gh.felicity.ac.cn/https://api.github.com/repos/gookit/gitw/releases/latest"
	if requested != want {
		t.Fatalf("expected ghproxy rewritten api url %q, got %q", want, requested)
	}
}

func TestGetWithOptionsFallsBackToNextGhproxyHost(t *testing.T) {
	origHTTPDo := httpDo
	defer func() { httpDo = origHTTPDo }()

	var requested []string
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		requested = append(requested, req.URL.String())
		if strings.Contains(req.URL.Host, "gh.felicity.ac.cn") {
			return nil, io.EOF
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	}

	resp, err := GetWithOptions("https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe", Options{
		GhproxyEnabled:   true,
		GhproxyHostURL:   "https://gh.felicity.ac.cn",
		GhproxyFallbacks: []string{"https://gh.llkk.cc"},
	})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	_ = resp.Body.Close()

	if len(requested) != 2 {
		t.Fatalf("expected 2 ghproxy attempts, got %#v", requested)
	}
	if !strings.Contains(requested[1], "gh.llkk.cc") {
		t.Fatalf("expected fallback ghproxy host, got %#v", requested)
	}
}

func TestOutputPathUsesHeuristicExecutableRename(t *testing.T) {
	file := ExtractedFile{Name: "chlog-windows-amd64.exe", mode: 0o666}
	got, err := outputPath(file, "", false, "")
	if err != nil {
		t.Fatalf("outputPath(): %v", err)
	}
	if got != "chlog.exe" {
		t.Fatalf("expected heuristic output name chlog.exe, got %q", got)
	}
}

func TestOutputPathUsesPreferredNameForExecutable(t *testing.T) {
	file := ExtractedFile{Name: "chlog-windows-amd64.exe", mode: 0o666}
	got, err := outputPath(file, "", false, "chlog")
	if err != nil {
		t.Fatalf("outputPath(): %v", err)
	}
	if got != "chlog.exe" {
		t.Fatalf("expected preferred output name chlog.exe, got %q", got)
	}
}

func TestOutputPathUsesPreferredNameWithExplicitExtension(t *testing.T) {
	file := ExtractedFile{Name: "chlog-windows-amd64.exe", mode: 0o666}
	got, err := outputPath(file, "", false, "custom-name.exe")
	if err != nil {
		t.Fatalf("outputPath(): %v", err)
	}
	if got != "custom-name.exe" {
		t.Fatalf("expected preferred explicit output name custom-name.exe, got %q", got)
	}
}

func TestOutputPathKeepsArchiveDirectoriesForExtractAll(t *testing.T) {
	file := ExtractedFile{Name: "Far/7-ZipEng.hlf", mode: 0o644}
	got, err := outputPath(file, "dist", true, "")
	if err != nil {
		t.Fatalf("outputPath(): %v", err)
	}
	want := filepath.Join("dist", "Far", "7-ZipEng.hlf")
	if got != want {
		t.Fatalf("expected extract-all output path %q, got %q", want, got)
	}
}

func TestOutputPathRejectsArchivePathTraversalForExtractAll(t *testing.T) {
	file := ExtractedFile{Name: "../evil.exe", mode: 0o644}
	if _, err := outputPath(file, "dist", true, ""); err == nil {
		t.Fatal("expected archive path traversal to be rejected")
	}
}

func TestAutoSelectExtractedFileByArch(t *testing.T) {
	candidates := []ExtractedFile{
		{ArchiveName: `arm64\WinDirStat.exe`, Name: `arm64\WinDirStat.exe`, mode: 0o666},
		{ArchiveName: `x86\WinDirStat.exe`, Name: `x86\WinDirStat.exe`, mode: 0o666},
		{ArchiveName: `x64\WinDirStat.exe`, Name: `x64\WinDirStat.exe`, mode: 0o666},
	}

	selected, ok := autoSelectExtractedFile(candidates, "windows", "amd64")
	if !ok {
		t.Fatal("expected auto selection for amd64 candidates")
	}
	if selected.ArchiveName != `x64\WinDirStat.exe` {
		t.Fatalf("expected x64 executable to be selected, got %q", selected.ArchiveName)
	}
}

func TestAutoSelectExtractedFilePicksOnlyWindowsExe(t *testing.T) {
	candidates := []ExtractedFile{
		{ArchiveName: `LICENSE`, Name: `LICENSE`, mode: 0o666},
		{ArchiveName: `gsa.exe`, Name: `gsa.exe`, mode: 0o666},
	}

	selected, ok := autoSelectExtractedFile(candidates, "windows", "amd64")
	if !ok {
		t.Fatal("expected auto selection for the only Windows executable")
	}
	if selected.ArchiveName != `gsa.exe` {
		t.Fatalf("expected gsa.exe to be selected, got %q", selected.ArchiveName)
	}
}

func TestAutoSelectExtractedFileKeepsPromptForMultipleWindowsExe(t *testing.T) {
	candidates := []ExtractedFile{
		{ArchiveName: `gsa.exe`, Name: `gsa.exe`, mode: 0o666},
		{ArchiveName: `gsa-helper.exe`, Name: `gsa-helper.exe`, mode: 0o666},
	}

	_, ok := autoSelectExtractedFile(candidates, "windows", "amd64")
	if ok {
		t.Fatal("expected multiple Windows executables to keep prompt fallback")
	}
}

func TestResolveExtractedFileUsesExplicitSystemForWindowsExeSelection(t *testing.T) {
	runner := &InstallRunner{Stderr: io.Discard}
	candidates := []ExtractedFile{
		{ArchiveName: `LICENSE`, Name: `LICENSE`, mode: 0o666},
		{ArchiveName: `gsa.exe`, Name: `gsa.exe`, mode: 0o666},
	}

	selected, all, err := runner.resolveExtractedFile(candidates, Options{System: "windows/amd64"})
	if err != nil {
		t.Fatalf("resolve extracted file: %v", err)
	}
	if all {
		t.Fatal("expected single file selection, got extract-all")
	}
	if selected.ArchiveName != `gsa.exe` {
		t.Fatalf("expected gsa.exe to be selected, got %q", selected.ArchiveName)
	}
}

func TestAutoSelectExtractedFileKeepsPromptWhenAmbiguous(t *testing.T) {
	candidates := []ExtractedFile{
		{ArchiveName: `bin\tool.exe`, Name: `bin\tool.exe`, mode: 0o666},
		{ArchiveName: `tools\tool.exe`, Name: `tools\tool.exe`, mode: 0o666},
	}

	_, ok := autoSelectExtractedFile(candidates, "windows", "amd64")
	if ok {
		t.Fatal("expected ambiguous candidates to keep prompt fallback")
	}
}
