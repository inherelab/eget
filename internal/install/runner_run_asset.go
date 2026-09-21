package install

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func validateInstallAction(opts Options) error {
	action := opts.URLTemplate.InstallAction
	if action == "" {
		return nil
	}
	if action != InstallActionRunAsset {
		return fmt.Errorf("unsupported install_action %q", action)
	}
	if opts.Verify == "" && opts.URLTemplate.ChecksumURLTemplate == "" {
		return fmt.Errorf("install_action %q requires checksum source", action)
	}
	return nil
}

func (r *InstallRunner) materializeRunAsset(source io.Reader, url string, opts Options) (string, error) {
	if IsLocalFile(url) {
		return url, nil
	}
	target := CacheFilePathWithMeta(opts.CacheDir, url, cacheMetaFromOptions(opts))
	if target == "" {
		target = filepath.Join(os.TempDir(), filepath.Base(url))
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	mode := os.FileMode(0o644)
	if runtime.GOOS != "windows" {
		mode = 0o755
	}
	if file, ok := source.(*os.File); ok && file.Name() == target {
		if runtime.GOOS != "windows" {
			_ = os.Chmod(target, mode)
		}
		return target, nil
	}
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
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
	if runtime.GOOS != "windows" {
		_ = os.Chmod(target, 0o755)
	}
	return target, nil
}

func (r *InstallRunner) runAsset(path string, args []string) error {
	stdout, stderr := r.Stdout, r.Stderr
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if r.AssetRunner != nil {
		return r.AssetRunner(path, append([]string(nil), args...), stdout, stderr)
	}
	cmd := exec.Command(path, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
