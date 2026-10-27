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
