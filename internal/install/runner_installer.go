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
	if err := launcher.LaunchInstaller(path, kind, opts.Silent); err != nil {
		return RunResult{}, err
	}
	// Asset is left for the caller: it must be the published asset name. The
	// materialized path is a cache file name (with version and URL hash) or an
	// archive member, neither of which matches the asset list later.
	return RunResult{
		IsGUI:         true,
		InstallMode:   InstallModeInstaller,
		InstallerFile: path,
	}, nil
}

func (r *InstallRunner) materializeInstallerFile(source io.Reader, url string, file ExtractedFile, opts Options, directAsset bool) (string, error) {
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
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(out, source)
	closeErr := out.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
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
