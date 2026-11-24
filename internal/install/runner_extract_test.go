package install

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
	"github.com/gookit/goutil/x/ccolor"
)

func TestRunExtractAllAppliesRenameGlobPattern(t *testing.T) {
	assetURL := "https://example.com/MobaXterm_Portable_v26.6.zip"
	archive := zipBytes(t, map[string]string{
		"MobaXterm_Personal_26.6.exe": "moba",
		"CygUtils.plugin":             "plugin",
	})
	outputDir := t.TempDir()

	svc := NewDefaultService(nil, nil)
	svc.SystemDetectorFactory = func(goos, goarch string) (Detector, error) {
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
		All:         true,
		Output:      outputDir,
		System:      "windows/amd64",
		RenameFiles: map[string]string{"MobaXterm_Personal_*.exe": "MobaXterm.exe"},
	})
	assert.NoErr(t, err)

	target := filepath.Join(outputDir, "MobaXterm.exe")
	info, err := os.Stat(target)
	assert.NoErr(t, err)
	assert.False(t, info.IsDir())
	if _, err := os.Stat(filepath.Join(outputDir, "MobaXterm_Personal_26.6.exe")); !os.IsNotExist(err) {
		t.Fatalf("expected the original asset name to be renamed, stat err=%v", err)
	}
	assert.Contains(t, result.ExtractedFiles, target)
}

func TestRunExtractAllWritesSingleFileToExplicitFileTarget(t *testing.T) {
	assetURL := "https://example.com/MobaXterm_Portable_v26.6.zip"
	target := filepath.Join(t.TempDir(), "MobaXterm.exe")
	direct := &writingDirectAllExtractor{files: map[string]string{
		"MobaXterm_Personal_26.6.exe": "moba",
	}}

	defer stubDownloadBody(t, []byte("archive"))()
	runner := NewRunner(directAllService(assetURL, direct))
	runner.Stdout = io.Discard
	runner.Stderr = io.Discard
	result, err := runner.Run(assetURL, Options{All: true, Output: target, OutputExplicit: true})
	assert.NoErr(t, err)

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("expected the extracted file at the file target: %v", err)
	}
	assert.False(t, info.IsDir())
	body, err := os.ReadFile(target)
	assert.NoErr(t, err)
	assert.Eq(t, "moba", string(body))
	assert.Eq(t, []string{target}, result.ExtractedFiles)
	entries, err := os.ReadDir(filepath.Dir(target))
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(entries))
}

func TestRunExtractAllRejectsMultipleFilesForExplicitFileTarget(t *testing.T) {
	assetURL := "https://example.com/MobaXterm_Portable_v26.6.zip"
	target := filepath.Join(t.TempDir(), "MobaXterm.exe")
	direct := &writingDirectAllExtractor{files: map[string]string{
		"MobaXterm_Personal_26.6.exe": "moba",
		"CygUtils.plugin":             "plugin",
	}}

	defer stubDownloadBody(t, []byte("archive"))()
	runner := NewRunner(directAllService(assetURL, direct))
	runner.Stdout = io.Discard
	runner.Stderr = io.Discard
	_, err := runner.Run(assetURL, Options{All: true, Output: target, OutputExplicit: true})

	assert.ErrMsgContains(t, err, "is a file target but 2 files were extracted")
}

func TestRunWarnsAboutUnmatchedRenamePattern(t *testing.T) {
	assetURL := "https://example.com/MobaXterm_Portable_v26.6.zip"
	archive := zipBytes(t, map[string]string{
		"MobaXterm_Personal_26.6.exe": "moba",
	})
	outputDir := t.TempDir()

	svc := NewDefaultService(nil, nil)
	svc.SystemDetectorFactory = func(goos, goarch string) (Detector, error) {
		return &fakeDetector{name: assetURL}, nil
	}
	defer stubDownloadBody(t, archive)()
	var stderr bytes.Buffer
	runner := NewRunner(svc)
	runner.Stdout = io.Discard
	runner.Stderr = &stderr
	_, err := runner.Run(assetURL, Options{
		All:         true,
		Output:      outputDir,
		System:      "windows/amd64",
		RenameFiles: map[string]string{"missing-*.dll": "missing.dll"},
	})
	assert.NoErr(t, err)
	assert.Contains(t, stderr.String(), "missing-*.dll")
	assert.Contains(t, stderr.String(), "matched no extracted file")
}

// stubDownloadBody serves body for every download request and restores the hook.
func stubDownloadBody(t *testing.T, body []byte) func() {
	t.Helper()
	origGetWithOptions := downloadGetWithOptions
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	}
	return func() { downloadGetWithOptions = origGetWithOptions }
}

// writingDirectAllExtractor creates the files it reports, like a real extract-all.
type writingDirectAllExtractor struct {
	files map[string]string
}

func (f *writingDirectAllExtractor) Extract(io.ReadSeeker, int64, bool) (ExtractedFile, []ExtractedFile, error) {
	return ExtractedFile{}, nil, nil
}

func (f *writingDirectAllExtractor) ExtractAllTo(_ io.ReadSeeker, _ int64, output string) ([]string, error) {
	paths := make([]string, 0, len(f.files))
	for name, body := range f.files {
		target := filepath.Join(output, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			return nil, err
		}
		paths = append(paths, target)
	}
	sort.Strings(paths)
	return paths, nil
}

func (f *writingDirectAllExtractor) ExtractAllToWithOptions(source io.ReadSeeker, size int64, output string, _ ArchiveExtractOptions) ([]string, error) {
	return f.ExtractAllTo(source, size, output)
}

func directAllService(assetURL string, direct *writingDirectAllExtractor) *Service {
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
	return svc
}

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

func TestRunExtractAllStripsAndRenamesSystem7zMember(t *testing.T) {
	assetURL := "https://example.com/tool.7z"
	outputDir := t.TempDir()
	origCommand := runSystem7zCommand
	defer func() { runSystem7zCommand = origCommand }()
	runSystem7zCommand = func(exe string, args ...string) ([]byte, error) {
		if args[0] != "x" {
			t.Fatalf("expected direct extract-all, got 7z %s", args[0])
		}
		for _, arg := range args {
			if strings.HasPrefix(arg, "-o") {
				root := strings.TrimPrefix(arg, "-o")
				for name, content := range map[string]string{
					"wrapper/exiftool(-k).exe": "exe",
					"wrapper/readme.txt":       "readme",
				} {
					file := filepath.Join(root, filepath.FromSlash(name))
					if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
						return nil, err
					}
					if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
						return nil, err
					}
				}
				return nil, nil
			}
		}
		t.Fatal("missing 7z output directory")
		return nil, nil
	}

	svc := NewDefaultService(nil, nil)
	svc.AllDetectorFactory = func() Detector { return &fakeDetector{name: assetURL} }
	svc.System7zPathResolver = func(string) string { return "7z" }
	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("archive"))}, nil
	}

	runner := NewRunner(svc)
	runner.Stdout = io.Discard
	runner.Stderr = io.Discard
	result, err := runner.Run(assetURL, Options{
		All: true, Output: outputDir, StripComponents: 1,
		RenameFiles: map[string]string{"exiftool(-k).exe": "exiftool.exe"},
	})
	if err != nil {
		t.Fatalf("run system 7z with strip and rename: %v", err)
	}
	assert.Eq(t, 2, len(result.ExtractedFiles))
	for name, want := range map[string]string{"exiftool.exe": "exe", "readme.txt": "readme"} {
		data, err := os.ReadFile(filepath.Join(outputDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		assert.Eq(t, want, string(data))
	}
	if _, err := os.Stat(filepath.Join(outputDir, "wrapper")); !os.IsNotExist(err) {
		t.Fatalf("expected stripped wrapper to be absent, stat err=%v", err)
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
