package install

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	"github.com/gookit/goutil/x/ccolor"
	sourcegithub "github.com/inherelab/eget/internal/source/github"
	"github.com/inherelab/eget/internal/source/urltemplate"
)

func TestRunDownloadOnlyPreservesLastModifiedTimestamp(t *testing.T) {
	modTime := time.Date(2026, 4, 30, 11, 32, 59, 0, time.UTC)
	body := []byte("asset-data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", modTime.Format(http.TimeFormat))
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	}))
	defer server.Close()

	outputDir := t.TempDir()
	runner := NewRunner(NewDefaultService(nil, nil))
	runner.Stdout = io.Discard
	runner.Stderr = io.Discard

	result, err := runner.Run(server.URL+"/rufus-4.14p.exe", Options{
		DownloadOnly: true,
		Output:       outputDir,
		CacheDir:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("download asset: %v", err)
	}

	assert.Eq(t, 1, len(result.ExtractedFiles))
	info, err := os.Stat(result.ExtractedFiles[0])
	if err != nil {
		t.Fatalf("stat downloaded file: %v", err)
	}
	assert.Eq(t, modTime, info.ModTime().UTC())
}

func TestRunDownloadOnlyPreservesLastModifiedTimestampWithoutCache(t *testing.T) {
	modTime := time.Date(2026, 4, 30, 11, 32, 59, 0, time.UTC)
	body := []byte("asset-data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", modTime.Format(http.TimeFormat))
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(body)
	}))
	defer server.Close()

	outputDir := t.TempDir()
	runner := NewRunner(NewDefaultService(nil, nil))
	runner.Stdout = io.Discard
	runner.Stderr = io.Discard

	result, err := runner.Run(server.URL+"/rufus-4.14p.exe", Options{
		DownloadOnly: true,
		Output:       outputDir,
	})
	if err != nil {
		t.Fatalf("download asset: %v", err)
	}

	assert.Eq(t, 1, len(result.ExtractedFiles))
	info, err := os.Stat(result.ExtractedFiles[0])
	if err != nil {
		t.Fatalf("stat downloaded file: %v", err)
	}
	assert.Eq(t, modTime, info.ModTime().UTC())
}

func TestRunDownloadOnlyTreatsTrailingSeparatorOutputAsDirectory(t *testing.T) {
	body := []byte("asset-data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	}))
	defer server.Close()

	outputDir := filepath.Join(t.TempDir(), "v1.1.7") + string(os.PathSeparator)
	runner := NewRunner(NewDefaultService(nil, nil))
	runner.Stdout = io.Discard
	runner.Stderr = io.Discard

	result, err := runner.Run(server.URL+"/default-arm64-v8a.jar", Options{
		DownloadOnly:   true,
		Output:         outputDir,
		OutputExplicit: true,
	})
	if err != nil {
		t.Fatalf("download asset: %v", err)
	}

	want := filepath.Join(outputDir, "default-arm64-v8a.jar")
	assert.Eq(t, []string{want}, result.ExtractedFiles)
	data, err := os.ReadFile(want)
	assert.NoErr(t, err)
	assert.Eq(t, body, data)
}

func TestRunExtractFilePreservesArchiveDirectoryTimestamp(t *testing.T) {
	dirTime := time.Date(2024, 4, 5, 6, 7, 8, 0, time.UTC)
	remoteTime := time.Date(2026, 4, 30, 11, 32, 59, 0, time.UTC)
	var body bytes.Buffer
	zw := zip.NewWriter(&body)
	dirHeader := &zip.FileHeader{Name: "CdmResource/", Method: zip.Store, Modified: dirTime}
	dirHeader.SetMode(0o755 | os.ModeDir)
	if _, err := zw.CreateHeader(dirHeader); err != nil {
		t.Fatalf("create zip dir: %v", err)
	}
	fileHeader := &zip.FileHeader{Name: "CdmResource/resource.txt", Method: zip.Store, Modified: dirTime}
	fileHeader.SetMode(0o644)
	w, err := zw.CreateHeader(fileHeader)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	if _, err := w.Write([]byte("resource")); err != nil {
		t.Fatalf("write zip file: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", remoteTime.Format(http.TimeFormat))
		w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
		_, _ = w.Write(body.Bytes())
	}))
	defer server.Close()

	outputDir := t.TempDir()
	runner := NewRunner(NewDefaultService(nil, nil))
	runner.Stdout = io.Discard
	runner.Stderr = io.Discard

	result, err := runner.Run(server.URL+"/app.zip", Options{
		DownloadOnly: true,
		ExtractFile:  "CdmResource",
		Output:       outputDir,
		CacheDir:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("extract directory: %v", err)
	}

	assert.Eq(t, 1, len(result.ExtractedFiles))
	info, err := os.Stat(filepath.Join(outputDir, "CdmResource"))
	if err != nil {
		t.Fatalf("stat extracted directory: %v", err)
	}
	assert.Eq(t, dirTime, info.ModTime().UTC())
}

func TestRunTemplateFinderResolvesChecksumVarsForVerifier(t *testing.T) {
	assetURL := "https://example.com/1.2.3/win32-x64/claude.exe"
	outputDir := t.TempDir()
	origDownloadGetWithOptions := downloadGetWithOptions
	origHTTPDo := httpDo
	defer func() {
		downloadGetWithOptions = origDownloadGetWithOptions
		httpDo = origHTTPDo
	}()

	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		assert.Eq(t, assetURL, url)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("binary-data")),
		}, nil
	}
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		_ = client
		assert.Eq(t, "https://example.com/1.2.3/manifest.json", req.URL.String())
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"platforms":{"win32-x64":{"checksum":"abc"}}}`)),
		}, nil
	}

	svc := NewService()
	svc.TemplateGetterFactory = func(opts Options) urltemplate.HTTPGetter {
		return fakeHTTPGetterFunc(func(url string) (*http.Response, error) {
			assert.Eq(t, "https://example.com/latest", url)
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader("1.2.3")),
			}, nil
		})
	}
	svc.AllDetectorFactory = func() Detector {
		return &fakeDetector{name: assetURL}
	}
	svc.SystemDetectorFactory = func(goos, goarch string) (Detector, error) {
		assert.Eq(t, "windows", goos)
		assert.Eq(t, "amd64", goarch)
		return &fakeDetector{name: assetURL}, nil
	}
	var verifierValue string
	svc.Sha256VerifierFactory = func(expected string) (Verifier, error) {
		verifierValue = expected
		return &fakeVerifier{}, nil
	}
	svc.BinaryChooserFactory = func(tool string) any {
		return &fakeChooser{name: tool}
	}
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		return fakeInstallExtractor{file: ExtractedFile{
			Name:        "claude.exe",
			ArchiveName: "claude.exe",
			Extract: func(to string) error {
				return os.WriteFile(to, []byte("binary-data"), 0o755)
			},
		}}
	}

	runner := NewRunner(svc)
	runner.Stdout = io.Discard
	runner.Stderr = io.Discard

	result, err := runner.Run("template:claude", Options{
		System: "windows/amd64",
		Output: outputDir,
		URLTemplate: URLTemplateOptions{
			LatestURL:           "https://example.com/latest",
			URLTemplate:         "https://example.com/{version}/{os}-{arch}/claude{ext}",
			OSMap:               map[string]string{"windows": "win32"},
			ArchMap:             map[string]string{"amd64": "x64"},
			ExtMap:              map[string]string{"windows": ".exe"},
			ChecksumURLTemplate: "https://example.com/{version}/manifest.json",
			ChecksumFormat:      "json",
			ChecksumJSONPath:    "platforms.{os}-{arch}.checksum",
		},
	})
	if err != nil {
		t.Fatalf("run template install: %v", err)
	}

	assert.Eq(t, "abc", verifierValue)
	assert.Eq(t, assetURL, result.URL)
	assert.Eq(t, "1.2.3", result.Version)
	assert.Eq(t, []string{filepath.Join(outputDir, "claude.exe")}, result.ExtractedFiles)
}

func TestRunTreatsConfiguredOutputAsDirectoryWhenMissing(t *testing.T) {
	assetURL := "https://example.com/mutagen_linux_amd64_v0.18.1.tar.gz"
	origDownloadGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origDownloadGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		assert.Eq(t, assetURL, url)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("archive-data")),
		}, nil
	}

	svc := NewService()
	svc.AllDetectorFactory = func() Detector {
		return &fakeDetector{name: assetURL}
	}
	svc.NoVerifierFactory = func() Verifier {
		return &fakeVerifier{}
	}
	svc.BinaryChooserFactory = func(tool string) any {
		return &fakeChooser{name: "mutagen"}
	}
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		return fakeInstallExtractor{file: ExtractedFile{
			Name:        "mutagen",
			ArchiveName: "mutagen",
			mode:        0o755,
			Extract: func(to string) error {
				if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
					return err
				}
				return os.WriteFile(to, []byte("binary-data"), 0o755)
			},
		}}
	}

	outputDir := filepath.Join(t.TempDir(), "bin")
	runner := NewRunner(svc)
	runner.Stdout = io.Discard
	runner.Stderr = io.Discard

	result, err := runner.Run(assetURL, Options{Output: outputDir})
	if err != nil {
		t.Fatalf("run install: %v", err)
	}

	want := filepath.Join(outputDir, "mutagen")
	assert.Eq(t, []string{want}, result.ExtractedFiles)
	data, err := os.ReadFile(want)
	assert.NoErr(t, err)
	assert.Eq(t, "binary-data", string(data))
	info, err := os.Stat(outputDir)
	assert.NoErr(t, err)
	assert.True(t, info.IsDir())
}

type failingVerifier struct{}

func (failingVerifier) Verify(io.Reader) error {
	return errors.New("checksum failed")
}

type fakeInstallExtractor struct {
	file ExtractedFile
}

func (f fakeInstallExtractor) Extract(io.ReadSeeker, int64, bool) (ExtractedFile, []ExtractedFile, error) {
	return f.file, nil, nil
}

type fakeDirectAllExtractor struct {
	extractCalled bool
	files         []string
	strip         int
}

func (f *fakeDirectAllExtractor) Extract(io.ReadSeeker, int64, bool) (ExtractedFile, []ExtractedFile, error) {
	f.extractCalled = true
	return ExtractedFile{}, nil, nil
}

func (f *fakeDirectAllExtractor) ExtractAllTo(io.ReadSeeker, int64, string) ([]string, error) {
	return f.files, nil
}

func (f *fakeDirectAllExtractor) ExtractAllToWithOptions(source io.ReadSeeker, size int64, output string, opts ArchiveExtractOptions) ([]string, error) {
	f.strip = opts.StripComponents
	return f.ExtractAllTo(source, size, output)
}

func TestRunAutoExtractsMultipleWindowsExecutables(t *testing.T) {
	assetURL := "https://example.com/uv.zip"
	archive := zipBytes(t, map[string]string{
		"uv.exe":      "uv",
		"uvx.exe":     "uvx",
		"uvw.exe":     "uvw",
		"README.md":   "readme",
		"LICENSE.txt": "license",
	})
	outputDir := t.TempDir()

	svc := NewDefaultService(nil, nil)
	svc.SystemDetectorFactory = func(goos, goarch string) (Detector, error) {
		assert.Eq(t, "windows", goos)
		assert.Eq(t, "amd64", goarch)
		return &fakeDetector{name: assetURL}, nil
	}
	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(archive)),
		}, nil
	}

	runner := NewRunner(svc)
	runner.Stdout = io.Discard
	runner.Stderr = io.Discard
	result, err := runner.Run(assetURL, Options{
		Name:   "uv",
		Output: outputDir,
		System: "windows/amd64",
	})
	if err != nil {
		t.Fatalf("run install: %v", err)
	}

	gotFiles := append([]string(nil), result.ExtractedFiles...)
	wantFiles := []string{
		filepath.Join(outputDir, "uv.exe"),
		filepath.Join(outputDir, "uvx.exe"),
		filepath.Join(outputDir, "uvw.exe"),
	}
	sort.Strings(gotFiles)
	sort.Strings(wantFiles)
	assert.Eq(t, wantFiles, gotFiles)
	for _, file := range result.ExtractedFiles {
		if _, err := os.Stat(file); err != nil {
			t.Fatalf("expected extracted file %s: %v", file, err)
		}
	}
	if _, err := os.Stat(filepath.Join(outputDir, "README.md")); !os.IsNotExist(err) {
		t.Fatalf("expected README.md not to be auto-extracted, stat err=%v", err)
	}
}

func TestRunAutoExtractsNestedPlatformExecutablesToOutputRoot(t *testing.T) {
	assetURL := "https://example.com/uv.zip"
	archive := zipBytesWithModes(t, map[string]zipTestFile{
		"uv-x86_64-unknown-linux-gnu/uv":        {body: "uv", mode: 0o755},
		"uv-x86_64-unknown-linux-gnu/uvx":       {body: "uvx", mode: 0o755},
		"uv-x86_64-unknown-linux-gnu/README.md": {body: "readme", mode: 0o644},
	})
	outputDir := t.TempDir()

	svc := NewDefaultService(nil, nil)
	svc.SystemDetectorFactory = func(goos, goarch string) (Detector, error) {
		assert.Eq(t, "linux", goos)
		assert.Eq(t, "amd64", goarch)
		return &fakeDetector{name: assetURL}, nil
	}
	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(archive)),
		}, nil
	}

	runner := NewRunner(svc)
	runner.Stdout = io.Discard
	runner.Stderr = io.Discard
	result, err := runner.Run(assetURL, Options{
		Name:   "uv",
		Output: outputDir,
		System: "linux/amd64",
	})
	if err != nil {
		t.Fatalf("run install: %v", err)
	}

	gotFiles := append([]string(nil), result.ExtractedFiles...)
	wantFiles := []string{
		filepath.Join(outputDir, "uv"),
		filepath.Join(outputDir, "uvx"),
	}
	sort.Strings(gotFiles)
	sort.Strings(wantFiles)
	assert.Eq(t, wantFiles, gotFiles)
	for _, file := range result.ExtractedFiles {
		if _, err := os.Stat(file); err != nil {
			t.Fatalf("expected extracted file %s: %v", file, err)
		}
	}
	if _, err := os.Stat(filepath.Join(outputDir, "uv-x86_64-unknown-linux-gnu", "uv")); !os.IsNotExist(err) {
		t.Fatalf("expected platform directory not to be created, stat err=%v", err)
	}
}

type zipTestFile struct {
	body string
	mode os.FileMode
}

func zipBytesWithModes(t *testing.T, files map[string]zipTestFile) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, file := range files {
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		header.SetMode(file.mode)
		w, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatalf("create zip file: %v", err)
		}
		if _, err := w.Write([]byte(file.body)); err != nil {
			t.Fatalf("write zip file: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		header.SetMode(0o644)
		w, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatalf("create zip file: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write zip file: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
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

func TestRunWritesSuccessfulInstallOutputToStdout(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "tool.exe")
	if err := os.WriteFile(source, []byte("tool"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}
	outputDir := filepath.Join(tmpDir, "bin")
	if err := os.Mkdir(outputDir, 0o755); err != nil {
		t.Fatalf("mkdir output: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := NewRunner(NewDefaultService(nil, nil))
	runner.Stdout = &stdout
	runner.Stderr = &stderr

	if _, err := runner.Run(source, Options{DownloadOnly: true, Output: outputDir}); err != nil {
		t.Fatalf("run install: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "Install") || !strings.Contains(got, "Asset") {
		t.Fatalf("expected successful install output on stdout, got %q", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected successful install stderr to be empty, got %q", got)
	}
}

func TestRunWritesUpdateVersionContextToStdout(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "xenv-windows-amd64.exe")
	if err := os.WriteFile(source, []byte("tool"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}
	outputDir := filepath.Join(tmpDir, "bin")
	if err := os.Mkdir(outputDir, 0o755); err != nil {
		t.Fatalf("mkdir output: %v", err)
	}

	var stdout bytes.Buffer
	runner := NewRunner(NewDefaultService(nil, nil))
	runner.Stdout = &stdout
	runner.Stderr = io.Discard

	if _, err := runner.Run(source, Options{
		DownloadOnly:   true,
		Output:         outputDir,
		Operation:      OperationUpdate,
		CurrentVersion: "0.2.0",
		TargetVersion:  "0.2.0-3-g3186bf3",
	}); err != nil {
		t.Fatalf("run update: %v", err)
	}

	got := stdout.String()
	assert.Contains(t, got, "Update")
	assert.Contains(t, got, "0.2.0")
	assert.Contains(t, got, "->")
	assert.Contains(t, got, "0.2.0-3-g3186bf3")
	assert.NotContains(t, got, "Install")
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
