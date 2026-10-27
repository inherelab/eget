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
)

func TestArchiveExtractorExtractAllToAutoStripsRedundantWrapperDir(t *testing.T) {
	out := t.TempDir()
	data := zipBytes(t, map[string]string{
		"dbdelve-0.5.1-windows-x86_64/dbdelve.exe":              "exe",
		"dbdelve-0.5.1-windows-x86_64/licenses/MIT-dbdelve.txt": "mit",
	})
	chooser, err := NewFileChooser("*")
	assert.NoErr(t, err)

	ex := NewArchiveExtractor(chooser, NewZipArchive, nil)
	files, err := ex.ExtractAllToWithOptions(bytes.NewReader(data), int64(len(data)), out, ArchiveExtractOptions{
		ArchiveName: "dbdelve-0.5.1-windows-x86_64.zip",
	})
	assert.NoErr(t, err)
	assert.Eq(t, 2, len(files))
	for name, want := range map[string]string{
		"dbdelve.exe":              "exe",
		"licenses/MIT-dbdelve.txt": "mit",
	} {
		got, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(name)))
		assert.NoErr(t, err)
		assert.Eq(t, want, string(got))
	}
	if _, err := os.Stat(filepath.Join(out, "dbdelve-0.5.1-windows-x86_64")); !os.IsNotExist(err) {
		t.Fatalf("expected redundant wrapper dir to be stripped, stat err=%v", err)
	}
}

func TestArchiveExtractorExtractAllToKeepsWrapperDirWhenNameDiffers(t *testing.T) {
	out := t.TempDir()
	data := zipBytes(t, map[string]string{
		"v2rayN-windows-64/v2rayN.exe": "exe",
	})
	chooser, err := NewFileChooser("*")
	assert.NoErr(t, err)

	ex := NewArchiveExtractor(chooser, NewZipArchive, nil)
	_, err = ex.ExtractAllToWithOptions(bytes.NewReader(data), int64(len(data)), out, ArchiveExtractOptions{
		ArchiveName: "v2rayN-windows-64-desktop.zip",
	})
	assert.NoErr(t, err)
	// The wrapper name carries information the asset name does not, so it stays.
	_, err = os.Stat(filepath.Join(out, "v2rayN-windows-64", "v2rayN.exe"))
	assert.NoErr(t, err)
}

func TestArchiveExtractorExtractAllToKeepsLayoutWithRootLevelEntries(t *testing.T) {
	out := t.TempDir()
	data := zipBytes(t, map[string]string{
		"dbdelve-0.5.1-windows-x86_64/dbdelve.exe": "exe",
		"README.md": "readme",
	})
	chooser, err := NewFileChooser("*")
	assert.NoErr(t, err)

	ex := NewArchiveExtractor(chooser, NewZipArchive, nil)
	files, err := ex.ExtractAllToWithOptions(bytes.NewReader(data), int64(len(data)), out, ArchiveExtractOptions{
		ArchiveName: "dbdelve-0.5.1-windows-x86_64.zip",
	})
	assert.NoErr(t, err)
	assert.Eq(t, 2, len(files))
	_, err = os.Stat(filepath.Join(out, "dbdelve-0.5.1-windows-x86_64", "dbdelve.exe"))
	assert.NoErr(t, err)
	_, err = os.Stat(filepath.Join(out, "README.md"))
	assert.NoErr(t, err)
}

func TestArchiveExtractorExtractAllToExplicitStripComponentsWins(t *testing.T) {
	out := t.TempDir()
	data := zipBytes(t, map[string]string{
		"dbdelve-0.5.1-windows-x86_64/bin/dbdelve.exe": "exe",
	})
	chooser, err := NewFileChooser("*")
	assert.NoErr(t, err)

	ex := NewArchiveExtractor(chooser, NewZipArchive, nil)
	files, err := ex.ExtractAllToWithOptions(bytes.NewReader(data), int64(len(data)), out, ArchiveExtractOptions{
		StripComponents: 2,
		ArchiveName:     "dbdelve-0.5.1-windows-x86_64.zip",
	})
	assert.NoErr(t, err)
	assert.Eq(t, []string{filepath.Join(out, "dbdelve.exe")}, files)
}

func TestArchiveExtractorExtractAllToKeepsAppBundleWrapper(t *testing.T) {
	out := t.TempDir()
	data := zipBytes(t, map[string]string{
		"Kero.app/Contents/Info.plist": "plist",
		"Kero.app/Contents/MacOS/kero": "bin",
	})
	chooser, err := NewFileChooser("*")
	assert.NoErr(t, err)

	ex := NewArchiveExtractor(chooser, NewZipArchive, nil)
	files, err := ex.ExtractAllToWithOptions(bytes.NewReader(data), int64(len(data)), out, ArchiveExtractOptions{
		ArchiveName: "Kero.app.zip",
	})
	assert.NoErr(t, err)
	// The bundle directory is the application itself even though it repeats the
	// asset name, so it must survive the redundant-wrapper stripping.
	assert.Eq(t, 2, len(files))
	_, err = os.Stat(filepath.Join(out, "Kero.app", "Contents", "Info.plist"))
	assert.NoErr(t, err)
}

func TestSystem7zExtractorExtractAllToAutoStripsRedundantWrapperDir(t *testing.T) {
	out := t.TempDir()
	origRunner := runSystem7zCommand
	defer func() { runSystem7zCommand = origRunner }()

	runSystem7zCommand = func(exe string, args ...string) ([]byte, error) {
		outDir := ""
		for _, arg := range args {
			if strings.HasPrefix(arg, "-o") {
				outDir = strings.TrimPrefix(arg, "-o")
			}
		}
		extracted := filepath.Join(outDir, "dbdelve-0.5.1-windows-x86_64", "dbdelve.exe")
		if err := os.MkdirAll(filepath.Dir(extracted), 0o755); err != nil {
			return nil, err
		}
		return nil, os.WriteFile(extracted, []byte("exe"), 0o644)
	}

	chooser, err := NewFileChooser("*")
	assert.NoErr(t, err)
	extractor := NewSystem7zExtractor("dbdelve-0.5.1-windows-x86_64.7z", "dbdelve", chooser, "7z")
	data := []byte("archive")
	files, err := extractor.ExtractAllToWithOptions(bytes.NewReader(data), int64(len(data)), out, ArchiveExtractOptions{
		ArchiveName: "dbdelve-0.5.1-windows-x86_64.7z",
	})
	assert.NoErr(t, err)
	assert.Eq(t, []string{filepath.Join(out, "dbdelve.exe")}, files)
}

func TestRunPortableGUIArchiveHoistsAppBundleToGuiTarget(t *testing.T) {
	assetURL := "https://example.com/kero-2.1.24-darwin-arm64.zip"
	archive := zipBytes(t, map[string]string{
		"Kero.app/Contents/Info.plist":      "plist",
		"Kero.app/Contents/MacOS/Kero":      "bin",
		"Kero.app/Contents/Resources/a.txt": "res",
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
		Name:        "kero",
		GuiTarget:   guiTarget,
		IsGUI:       true,
		InstallMode: InstallModePortable,
	})
	if err != nil {
		t.Fatalf("run portable gui app bundle: %v", err)
	}

	bundle := filepath.Join(guiTarget, "Kero.app")
	assert.Eq(t, 3, len(result.ExtractedFiles))
	for _, name := range []string{
		filepath.Join(bundle, "Contents", "Info.plist"),
		filepath.Join(bundle, "Contents", "MacOS", "Kero"),
		filepath.Join(bundle, "Contents", "Resources", "a.txt"),
	} {
		if _, err := os.Stat(name); err != nil {
			t.Fatalf("expected hoisted bundle file %s: %v", name, err)
		}
		assert.Contains(t, strings.Join(result.ExtractedFiles, "\n"), name)
	}
	if _, err := os.Stat(filepath.Join(guiTarget, "kero")); !os.IsNotExist(err) {
		t.Fatalf("expected the injected <gui_target>/<name> level to be removed, stat err=%v", err)
	}
}

func TestRunPortableGUIArchiveKeepsBundleUnderExplicitOutput(t *testing.T) {
	assetURL := "https://example.com/kero-2.1.24-darwin-arm64.zip"
	archive := zipBytes(t, map[string]string{
		"Kero.app/Contents/Info.plist": "plist",
	})
	applications := t.TempDir()

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
	_, err := runner.Run(assetURL, Options{
		Name:           "kero",
		Output:         applications,
		OutputExplicit: true,
		GuiTarget:      applications,
		IsGUI:          true,
		InstallMode:    InstallModePortable,
	})
	if err != nil {
		t.Fatalf("run portable gui app bundle to explicit output: %v", err)
	}

	if _, err := os.Stat(filepath.Join(applications, "Kero.app", "Contents", "Info.plist")); err != nil {
		t.Fatalf("expected bundle directly under the explicit output: %v", err)
	}
}

func TestRunPortableGUIArchiveAutoStripsWrapperDirRepeatingAssetName(t *testing.T) {
	assetURL := "https://example.com/dbdelve-0.5.1-windows-x86_64.zip"
	archive := zipBytes(t, map[string]string{
		"dbdelve-0.5.1-windows-x86_64/dbdelve.exe":              "exe",
		"dbdelve-0.5.1-windows-x86_64/licenses/MIT-dbdelve.txt": "mit",
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
		Name:        "dbdelve",
		GuiTarget:   guiTarget,
		IsGUI:       true,
		InstallMode: InstallModePortable,
	})
	if err != nil {
		t.Fatalf("run portable gui archive: %v", err)
	}

	wantRoot := filepath.Join(guiTarget, "dbdelve")
	assert.Eq(t, 2, len(result.ExtractedFiles))
	for _, name := range []string{
		filepath.Join(wantRoot, "dbdelve.exe"),
		filepath.Join(wantRoot, "licenses", "MIT-dbdelve.txt"),
	} {
		if _, err := os.Stat(name); err != nil {
			t.Fatalf("expected extracted file %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(wantRoot, "dbdelve-0.5.1-windows-x86_64")); !os.IsNotExist(err) {
		t.Fatalf("expected redundant wrapper dir to be stripped, stat err=%v", err)
	}
}
