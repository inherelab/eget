package install

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (r *InstallRunner) confirmLaunchInstaller(file string) (bool, error) {
	confirm := r.ConfirmLaunchInstaller
	if confirm == nil {
		confirm = defaultConfirmLaunchInstaller
	}
	return confirm(filepath.Base(file))
}

func defaultConfirmLaunchInstaller(file string) (bool, error) {
	fmt.Fprintf(os.Stderr, "%s looks like a GUI installer. Launch it now? [y/N]: ", file)
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		if err == io.EOF && strings.TrimSpace(answer) == "" {
			return false, nil
		}
		if err != io.EOF {
			return false, err
		}
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

func (r *InstallRunner) launchGUIInstaller(path string, file ExtractedFile, opts Options) (RunResult, error) {
	kind := DetectInstallerKind(file.ArchiveName)
	if kind == InstallerKindUnknown {
		kind = DetectInstallerKind(file.Name)
	}
	if kind == InstallerKindUnknown && opts.InstallMode == InstallModeInstaller {
		kind = InstallerKindEXE
	}
	launcher := r.InstallerLauncher
	if launcher == nil {
		launcher = DefaultInstallerLauncher{}
	}
	if err := launcher.LaunchInstaller(path, kind); err != nil {
		return RunResult{}, err
	}
	return RunResult{
		Asset:         filepath.Base(path),
		IsGUI:         true,
		InstallMode:   InstallModeInstaller,
		InstallerFile: path,
	}, nil
}

func (r *InstallRunner) materializeInstallerFile(body []byte, url string, file ExtractedFile, opts Options, directAsset bool) (string, error) {
	if IsLocalFile(url) {
		return url, nil
	}
	if cachePath := CacheFilePathWithMeta(opts.CacheDir, url, cacheMetaFromOptions(opts)); directAsset && cachePath != "" {
		if _, err := os.Stat(cachePath); err == nil {
			return cachePath, nil
		}
	}

	target := installerMaterializePath(opts, file)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if file.Extract != nil {
		if err := file.Extract(target); err != nil {
			return "", err
		}
		return target, nil
	}
	if err := os.WriteFile(target, body, 0o755); err != nil {
		return "", err
	}
	return target, nil
}

func installerMaterializePath(opts Options, file ExtractedFile) string {
	dir := opts.CacheDir
	if dir == "" {
		dir = os.TempDir()
	}

	rawName := file.Name
	if rawName == "" {
		rawName = file.ArchiveName
	}

	name := "installer"
	if rawName != "" {
		if safeName, err := safeArchiveRelativePath(rawName); err == nil && safeName != "" {
			name = safeName
		}
	}

	return filepath.Join(dir, "installers", filepath.Base(name))
}
