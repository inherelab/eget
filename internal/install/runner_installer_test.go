package install

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

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

func TestRunLaunchesConfiguredInstallerModeForPlainExe(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "GoNavi-0.7.7-Windows-Amd64.exe")
	if err := os.WriteFile(path, []byte("installer"), 0o755); err != nil {
		t.Fatalf("write installer: %v", err)
	}

	launcher := &fakeInstallerLauncher{}
	runner := NewRunner(NewDefaultService(nil, nil))
	runner.InstallerLauncher = launcher
	runner.Stderr = io.Discard
	runner.ConfirmLaunchInstaller = func(file string) (bool, error) {
		t.Fatalf("configured installer mode should not prompt, got %q", file)
		return false, nil
	}

	result, err := runner.Run(path, Options{IsGUI: true, InstallMode: InstallModeInstaller})
	assert.NoErr(t, err)
	assert.Eq(t, path, launcher.path)
	assert.Eq(t, InstallerKindEXE, launcher.kind)
	assert.True(t, result.IsGUI)
	assert.Eq(t, InstallModeInstaller, result.InstallMode)
}

func TestRunGUIPlainExeDefaultsToInstaller(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "GoNavi-0.7.7-Windows-Amd64.exe")
	if err := os.WriteFile(path, []byte("installer"), 0o755); err != nil {
		t.Fatalf("write installer: %v", err)
	}

	launcher := &fakeInstallerLauncher{}
	runner := NewRunner(NewDefaultService(nil, nil))
	runner.InstallerLauncher = launcher
	runner.Stderr = io.Discard
	runner.ConfirmLaunchInstaller = func(file string) (bool, error) {
		t.Fatalf("gui plain exe should not prompt, got %q", file)
		return false, nil
	}

	result, err := runner.Run(path, Options{IsGUI: true})
	assert.NoErr(t, err)
	assert.Eq(t, path, launcher.path)
	assert.Eq(t, InstallerKindEXE, launcher.kind)
	assert.True(t, result.IsGUI)
	assert.Eq(t, InstallModeInstaller, result.InstallMode)
}

func TestRunGUIPortableExeUsesPortableMode(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "GoNavi-0.7.7-Windows-Amd64-portable.exe")
	if err := os.WriteFile(path, []byte("portable"), 0o755); err != nil {
		t.Fatalf("write portable exe: %v", err)
	}

	launcher := &fakeInstallerLauncher{}
	runner := NewRunner(NewDefaultService(nil, nil))
	runner.InstallerLauncher = launcher
	runner.Stderr = io.Discard
	outDir := t.TempDir()

	result, err := runner.Run(path, Options{IsGUI: true, Output: outDir})
	assert.NoErr(t, err)
	assert.Eq(t, "", launcher.path)
	assert.True(t, result.IsGUI)
	assert.Eq(t, InstallModePortable, result.InstallMode)
	assert.Eq(t, 1, len(result.ExtractedFiles))
	assert.Eq(t, filepath.Join(outDir, filepath.Base(path)), result.ExtractedFiles[0])
}

func TestRunExtractAllDoesNotPromptForDetectedInstaller(t *testing.T) {
	assetURL := "https://example.com/PicoClaw-Setup.exe"
	svc := NewService()
	svc.AllDetectorFactory = func() Detector {
		return &fakeDetector{name: assetURL}
	}
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		return fakeInstallExtractor{file: ExtractedFile{
			Name:        "PicoClaw-Setup.exe",
			ArchiveName: "PicoClaw-Setup.exe",
			Extract: func(to string) error {
				if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
					return err
				}
				return os.WriteFile(to, []byte("installer"), 0o755)
			},
		}}
	}
	svc.GlobChooserFactory = func(pattern string) (any, error) {
		return &fakeChooser{name: pattern}, nil
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
	var prompted bool
	runner.ConfirmLaunchInstaller = func(file string) (bool, error) {
		prompted = true
		return true, nil
	}

	result, err := runner.Run(assetURL, Options{All: true, Output: t.TempDir()})
	if err != nil {
		t.Fatalf("run extract-all installer-looking file: %v", err)
	}
	if prompted {
		t.Fatal("expected extract-all to skip installer launch prompt")
	}
	if result.IsGUI || result.InstallMode == InstallModeInstaller {
		t.Fatalf("expected extracted file result, got %#v", result)
	}
	if len(result.ExtractedFiles) != 1 {
		t.Fatalf("expected extracted file, got %#v", result.ExtractedFiles)
	}
}

func TestRunDirectInstallerUsesCacheFile(t *testing.T) {
	assetURL := "https://example.com/PowerShell-7.6.3-win-x64.msi"
	cacheDir := t.TempDir()
	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("installer")),
		}, nil
	}

	launcher := &fakeInstallerLauncher{}
	runner := NewRunner(NewDefaultService(nil, nil))
	runner.InstallerLauncher = launcher
	runner.Stdout = io.Discard
	runner.Stderr = io.Discard
	runner.ConfirmLaunchInstaller = func(file string) (bool, error) { return true, nil }

	_, err := runner.Run(assetURL, Options{
		CacheDir:    cacheDir,
		IsGUI:       true,
		InstallMode: InstallModeInstaller,
	})
	assert.NoErr(t, err)
	expected := CacheFilePathWithMeta(cacheDir, assetURL, CacheMeta{})
	assert.Eq(t, expected, launcher.path)
	assert.Eq(t, InstallerKindMSI, launcher.kind)
	_, statErr := os.Stat(filepath.Join(cacheDir, "installers"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestRunInstallerArchiveMaterializesSelectedFile(t *testing.T) {
	assetURL := "https://example.com/tool.zip"
	cacheDir := t.TempDir()
	svc := NewService()
	svc.AllDetectorFactory = func() Detector { return &fakeDetector{name: assetURL} }
	svc.ExtractorFactory = func(filename, tool string, chooser any) any {
		return fakeInstallExtractor{file: ExtractedFile{
			Name:        "Setup.exe",
			ArchiveName: "Setup.exe",
			Extract: func(to string) error {
				return os.WriteFile(to, []byte("installer"), 0o755)
			},
		}}
	}
	svc.BinaryChooserFactory = func(tool string) any { return &fakeChooser{name: tool} }
	svc.NoVerifierFactory = func() Verifier { return &fakeVerifier{} }

	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("archive")),
		}, nil
	}

	launcher := &fakeInstallerLauncher{}
	runner := NewRunner(svc)
	runner.InstallerLauncher = launcher
	runner.Stdout = io.Discard
	runner.Stderr = io.Discard
	runner.ConfirmLaunchInstaller = func(file string) (bool, error) { return true, nil }

	_, err := runner.Run(assetURL, Options{CacheDir: cacheDir})
	assert.NoErr(t, err)
	assert.Eq(t, filepath.Join(cacheDir, "installers", "Setup.exe"), launcher.path)
	assert.Eq(t, InstallerKindEXE, launcher.kind)
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
