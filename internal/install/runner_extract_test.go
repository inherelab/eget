package install

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
	"github.com/gookit/goutil/x/ccolor"
)

func TestRunExtractAllUsesDirectExtractorWithoutListing(t *testing.T) {
	assetURL := "https://example.com/tool.7z"
	outputDir := t.TempDir()
	direct := &fakeDirectAllExtractor{
		files: []string{filepath.Join(outputDir, "$PLUGINSDIR", "nsDialogs.dll")},
	}

	svc := NewService()
	svc.AllDetectorFactory = func() Detector {
		return &fakeDetector{name: assetURL}
	}
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		return direct
	}
	svc.System7zPathResolver = func(configured string) string {
		return ""
	}
	svc.GlobChooserFactory = func(pattern string) (any, error) {
		return NewFileChooser(pattern)
	}
	svc.NoVerifierFactory = func() Verifier {
		return &fakeVerifier{}
	}

	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("installer")),
		}, nil
	}

	runner := NewRunner(svc)
	runner.Stderr = io.Discard
	result, err := runner.Run(assetURL, Options{All: true, Output: outputDir})
	if err != nil {
		t.Fatalf("run extract-all: %v", err)
	}

	if direct.extractCalled {
		t.Fatal("expected direct extract-all to skip Extract")
	}
	if len(result.ExtractedFiles) != 1 {
		t.Fatalf("expected extracted files, got %#v", result.ExtractedFiles)
	}
}

func TestRunExtractAllPassesStripComponentsToDirectExtractor(t *testing.T) {
	assetURL := "https://example.com/ventoy.zip"
	outputDir := t.TempDir()
	direct := &fakeDirectAllExtractor{
		files: []string{filepath.Join(outputDir, "Ventoy2Disk.exe")},
	}

	svc := NewService()
	svc.AllDetectorFactory = func() Detector {
		return &fakeDetector{name: assetURL}
	}
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		return direct
	}
	svc.System7zPathResolver = func(configured string) string {
		return ""
	}
	svc.GlobChooserFactory = func(pattern string) (any, error) {
		return NewFileChooser(pattern)
	}
	svc.NoVerifierFactory = func() Verifier {
		return &fakeVerifier{}
	}

	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("zip")),
		}, nil
	}

	runner := NewRunner(svc)
	runner.Stderr = io.Discard
	_, err := runner.Run(assetURL, Options{All: true, Output: outputDir, StripComponents: 1})
	if err != nil {
		t.Fatalf("run extract-all: %v", err)
	}

	assert.Eq(t, 1, direct.strip)
}

func TestRunExtractAllStripsComponentsFromZipArchive(t *testing.T) {
	assetURL := "https://example.com/ventoy.zip"
	archive := zipBytes(t, map[string]string{
		"ventoy-1.1.12/Ventoy2Disk.exe": "exe",
		"ventoy-1.1.12/boot/boot.img":   "boot",
	})
	outputDir := t.TempDir()

	svc := NewDefaultService(nil, nil)
	svc.AllDetectorFactory = func() Detector {
		return &fakeDetector{name: assetURL}
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
	result, err := runner.Run(assetURL, Options{All: true, Output: outputDir, StripComponents: 1})
	if err != nil {
		t.Fatalf("run extract-all: %v", err)
	}

	assert.Eq(t, 2, len(result.ExtractedFiles))
	data, err := os.ReadFile(filepath.Join(outputDir, "Ventoy2Disk.exe"))
	if err != nil {
		t.Fatalf("read stripped exe: %v", err)
	}
	assert.Eq(t, "exe", string(data))
	data, err = os.ReadFile(filepath.Join(outputDir, "boot", "boot.img"))
	if err != nil {
		t.Fatalf("read stripped boot file: %v", err)
	}
	assert.Eq(t, "boot", string(data))
	if _, err := os.Stat(filepath.Join(outputDir, "ventoy-1.1.12")); !os.IsNotExist(err) {
		t.Fatalf("expected stripped root directory to be absent, stat err=%v", err)
	}
}

func TestRunExtractAllStripsComponentsAndRenamesZipMember(t *testing.T) {
	assetURL := "https://example.com/exiftool.zip"
	archive := zipBytes(t, map[string]string{
		"exiftool-13.40/exiftool(-k).exe": "exe",
		"exiftool-13.40/readme.txt":       "readme",
	})
	outputDir := t.TempDir()

	svc := NewDefaultService(nil, nil)
	svc.AllDetectorFactory = func() Detector {
		return &fakeDetector{name: assetURL}
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
		All: true, Output: outputDir, StripComponents: 1,
		RenameFiles: map[string]string{"exiftool(-k).exe": "exiftool.exe"},
	})
	if err != nil {
		t.Fatalf("run extract-all with strip and rename: %v", err)
	}

	assert.Eq(t, 2, len(result.ExtractedFiles))
	data, err := os.ReadFile(filepath.Join(outputDir, "exiftool.exe"))
	if err != nil {
		t.Fatalf("read renamed exe: %v", err)
	}
	assert.Eq(t, "exe", string(data))
	data, err = os.ReadFile(filepath.Join(outputDir, "readme.txt"))
	if err != nil {
		t.Fatalf("read stripped readme: %v", err)
	}
	assert.Eq(t, "readme", string(data))
	if _, err := os.Stat(filepath.Join(outputDir, "exiftool-13.40")); !os.IsNotExist(err) {
		t.Fatalf("expected stripped root directory to be absent, stat err=%v", err)
	}

	_, err = runner.Run(assetURL, Options{
		All: true, Output: t.TempDir(), StripComponents: 1,
		RenameFiles: map[string]string{"exiftool(-k).exe": "../escaped.exe"},
	})
	if err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("expected unsafe rename to be rejected, got %v", err)
	}
}

func TestRunExtractAllPrintsSingleHeaderForMultipleFiles(t *testing.T) {
	assetURL := "https://example.com/ventoy.zip"
	archive := zipBytes(t, map[string]string{
		"Ventoy2Disk.exe": "exe",
		"boot.img":        "boot",
	})
	outputDir := t.TempDir()

	svc := NewDefaultService(nil, nil)
	svc.AllDetectorFactory = func() Detector {
		return &fakeDetector{name: assetURL}
	}
	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(archive)),
		}, nil
	}

	var stdout bytes.Buffer
	runner := NewRunner(svc)
	runner.Stdout = &stdout
	runner.Stderr = io.Discard
	_, err := runner.Run(assetURL, Options{All: true, Output: outputDir})
	if err != nil {
		t.Fatalf("run extract-all: %v", err)
	}

	got := ccolor.ClearCode(stdout.String())
	assert.Contains(t, got, "✅ Extracted files:")
	assert.Eq(t, 1, strings.Count(got, "✅ Extracted"))
	assert.Contains(t, got, " - "+filepath.Join(outputDir, "Ventoy2Disk.exe"))
	assert.Contains(t, got, " - "+filepath.Join(outputDir, "boot.img"))
}

func TestRunExtractAllUsesContentDispositionFilenameForArchiveType(t *testing.T) {
	assetURL := "https://example.com/raw/artifact?job=build_linux"
	archive := zipBytes(t, map[string]string{
		"the-skeleton-key/readme.txt": "readme",
	})
	outputDir := t.TempDir()

	svc := NewDefaultService(nil, nil)
	svc.AllDetectorFactory = func() Detector {
		return &fakeDetector{name: assetURL}
	}
	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Disposition": []string{`attachment; filename="the-skeleton-key-linux.zip"`}},
			Body:       io.NopCloser(bytes.NewReader(archive)),
		}, nil
	}

	runner := NewRunner(svc)
	runner.Stdout = io.Discard
	runner.Stderr = io.Discard
	result, err := runner.Run(assetURL, Options{All: true, Output: outputDir})
	if err != nil {
		t.Fatalf("run extract-all content-disposition archive: %v", err)
	}

	assert.Eq(t, "the-skeleton-key-linux.zip", result.Asset)
	data, err := os.ReadFile(filepath.Join(outputDir, "the-skeleton-key", "readme.txt"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	assert.Eq(t, "readme", string(data))
}

func TestRunFileGlobWithStripComponentsExtractsStrippedMatches(t *testing.T) {
	assetURL := "https://example.com/ffmpeg.zip"
	archive := zipBytes(t, map[string]string{
		"ffmpeg-master-latest-win64-gpl/bin/ffmpeg.exe":  "ffmpeg",
		"ffmpeg-master-latest-win64-gpl/bin/ffplay.exe":  "ffplay",
		"ffmpeg-master-latest-win64-gpl/bin/ffprobe.exe": "ffprobe",
		"ffmpeg-master-latest-win64-gpl/doc/readme.txt":  "readme",
	})
	outputDir := t.TempDir()

	svc := NewDefaultService(nil, nil)
	svc.AllDetectorFactory = func() Detector {
		return &fakeDetector{name: assetURL}
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
	result, err := runner.Run(assetURL, Options{ExtractFile: "*.exe", All: true, Output: outputDir, StripComponents: 2})
	if err != nil {
		t.Fatalf("run file glob with strip-components: %v", err)
	}

	assert.Eq(t, 3, len(result.ExtractedFiles))
	for name, want := range map[string]string{
		"ffmpeg.exe":  "ffmpeg",
		"ffplay.exe":  "ffplay",
		"ffprobe.exe": "ffprobe",
	} {
		data, err := os.ReadFile(filepath.Join(outputDir, name))
		if err != nil {
			t.Fatalf("read extracted %s: %v", name, err)
		}
		assert.Eq(t, want, string(data))
	}
	if _, err := os.Stat(filepath.Join(outputDir, "ffmpeg-master-latest-win64-gpl")); !os.IsNotExist(err) {
		t.Fatalf("expected stripped root directory to be absent, stat err=%v", err)
	}
}

func TestRunPortableGUIArchiveExtractsAllToNamedGuiTarget(t *testing.T) {
	assetURL := "https://example.com/v2rayN-windows-64-desktop.zip"
	archive := zipBytes(t, map[string]string{
		"v2rayN-windows-64/v2rayN.exe":             "exe",
		"v2rayN-windows-64/bin/xray/xray.exe":      "xray",
		"v2rayN-windows-64/guiConfigs/config.json": "config",
	})
	guiTarget := t.TempDir()

	svc := NewDefaultService(nil, nil)
	svc.AllDetectorFactory = func() Detector {
		return &fakeDetector{name: assetURL}
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
		Name:        "v2rayn",
		GuiTarget:   guiTarget,
		IsGUI:       true,
		InstallMode: InstallModePortable,
	})
	if err != nil {
		t.Fatalf("run portable gui archive: %v", err)
	}

	wantRoot := filepath.Join(guiTarget, "v2rayn")
	assert.Eq(t, InstallModePortable, result.InstallMode)
	assert.True(t, result.IsGUI)
	assert.Eq(t, 3, len(result.ExtractedFiles))
	for _, name := range []string{
		filepath.Join(wantRoot, "v2rayN-windows-64", "v2rayN.exe"),
		filepath.Join(wantRoot, "v2rayN-windows-64", "bin", "xray", "xray.exe"),
		filepath.Join(wantRoot, "v2rayN-windows-64", "guiConfigs", "config.json"),
	} {
		if _, err := os.Stat(name); err != nil {
			t.Fatalf("expected extracted file %s: %v", name, err)
		}
	}
}
