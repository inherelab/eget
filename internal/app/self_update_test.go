package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gookit/goutil/x/assert"
	"github.com/inherelab/eget/internal/install"
)

func TestSelfUpdateSkipsWhenLatestMatchesCurrentVersion(t *testing.T) {
	svc := SelfUpdateService{
		CurrentVersion: "1.7.1",
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			assert.Eq(t, "inherelab/eget", target.Repo)
			return LatestInfo{Tag: "v1.7.1"}, nil
		},
	}

	result, err := svc.Update(SelfUpdateOptions{CheckOnly: true})

	assert.NoErr(t, err)
	assert.False(t, result.Updated)
	assert.False(t, result.Outdated)
	assert.Eq(t, "v1.7.1", result.LatestVersion)
}

func TestSelfUpdateReportsOutdatedWhenLatestDiffers(t *testing.T) {
	svc := SelfUpdateService{
		CurrentVersion: "1.7.1",
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v1.7.2"}, nil
		},
	}

	result, err := svc.Update(SelfUpdateOptions{CheckOnly: true})

	assert.NoErr(t, err)
	assert.True(t, result.Outdated)
	assert.Eq(t, "1.7.1", result.CurrentVersion)
	assert.Eq(t, "v1.7.2", result.LatestVersion)
}

func TestSelfUpdateDoesNotReportGitDescribeBuildFromLatestTagAsOutdated(t *testing.T) {
	svc := SelfUpdateService{
		CurrentVersion: "1.7.1-16-gc43a587",
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v1.7.1"}, nil
		},
	}

	result, err := svc.Update(SelfUpdateOptions{CheckOnly: true})

	assert.NoErr(t, err)
	assert.False(t, result.Outdated)
	assert.Eq(t, "v1.7.1", result.LatestVersion)
}

func TestSelfUpdateReportsGitDescribeBuildFromOlderTagAsOutdated(t *testing.T) {
	svc := SelfUpdateService{
		CurrentVersion: "1.7.0-3-gc43a587",
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v1.7.1"}, nil
		},
	}

	result, err := svc.Update(SelfUpdateOptions{CheckOnly: true})

	assert.NoErr(t, err)
	assert.True(t, result.Outdated)
	assert.Eq(t, "v1.7.1", result.LatestVersion)
}

func TestSelfUpdateDoesNotReportNewerGitDescribeBuildAsOutdated(t *testing.T) {
	assert.False(t, selfVersionOutdated("1.7.1-18-gabcd1234", "1.7.1-17-g0b8e439"))
}

func TestSelfUpdateDoesNotReportNewerReleaseBuildAsOutdated(t *testing.T) {
	assert.False(t, selfVersionOutdated("v1.7.2", "v1.7.1"))
}

func TestSelfUpdateDownloadsExpectedPlatformAsset(t *testing.T) {
	replacement := filepath.Join(t.TempDir(), "eget")
	assert.NoErr(t, os.WriteFile(replacement, []byte("new"), 0o755))
	installer := &fakeSelfUpdateInstaller{
		result: RunResult{ExtractedFiles: []string{replacement}},
	}
	svc := SelfUpdateService{
		CurrentVersion: "1.7.1",
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v1.7.2"}, nil
		},
		Installer:     installer,
		RuntimeGOOS:   "linux",
		RuntimeGOARCH: "amd64",
		ExecutablePath: func() (string, error) {
			return filepath.Join(t.TempDir(), "eget"), nil
		},
		Replacer: fakeSelfReplacer{},
	}

	_, err := svc.Update(SelfUpdateOptions{})

	assert.NoErr(t, err)
	assert.Eq(t, SelfUpdateRepo, installer.target)
	assert.Eq(t, "linux/amd64", installer.opts.System)
	assert.Eq(t, "eget-linux-amd64", installer.opts.ExtractFile)
	assert.False(t, installer.opts.DownloadOnly)
	assert.Eq(t, "eget", installer.opts.Name)
	assert.Eq(t, 0, len(installer.opts.Asset))
}

func TestSelfUpdateAllowsExplicitAssetOverride(t *testing.T) {
	replacement := filepath.Join(t.TempDir(), "eget")
	assert.NoErr(t, os.WriteFile(replacement, []byte("new"), 0o755))
	installer := &fakeSelfUpdateInstaller{
		result: RunResult{ExtractedFiles: []string{replacement}},
	}
	svc := SelfUpdateService{
		CurrentVersion: "1.7.1",
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v1.7.2"}, nil
		},
		Installer:     installer,
		RuntimeGOOS:   "linux",
		RuntimeGOARCH: "amd64",
		ExecutablePath: func() (string, error) {
			return filepath.Join(t.TempDir(), "eget"), nil
		},
		Replacer: fakeSelfReplacer{},
	}

	_, err := svc.Update(SelfUpdateOptions{Asset: []string{"REG:eget_.*linux_amd64"}})

	assert.NoErr(t, err)
	assert.Eq(t, []string{"REG:eget_.*linux_amd64"}, installer.opts.Asset)
}

func TestSelfUpdateDownloadsFromInternalSource(t *testing.T) {
	replacement := filepath.Join(t.TempDir(), "eget")
	assert.NoErr(t, os.WriteFile(replacement, []byte("new"), 0o755))
	installer := &fakeSelfUpdateInstaller{
		result: RunResult{ExtractedFiles: []string{replacement}},
	}
	svc := SelfUpdateService{
		CurrentVersion: "1.7.1-17-g0b8e439",
		SourceLatestInfo: func(source string, opts install.Options) (LatestInfo, error) {
			assert.Eq(t, "https://example.com/tools/eget/", source)
			return LatestInfo{Tag: "1.7.1-18-gabcd1234"}, nil
		},
		Installer:     installer,
		Replacer:      fakeSelfReplacer{},
		RuntimeGOOS:   "linux",
		RuntimeGOARCH: "amd64",
		ExecutablePath: func() (string, error) {
			return filepath.Join(t.TempDir(), "eget"), nil
		},
	}

	result, err := svc.Update(SelfUpdateOptions{Source: "https://example.com/tools/eget/"})

	assert.NoErr(t, err)
	assert.True(t, result.Updated)
	assert.Eq(t, "1.7.1-18-gabcd1234", result.LatestVersion)
	assert.Eq(t, "https://example.com/tools/eget/eget-linux-amd64", installer.target)
	assert.Eq(t, "eget", installer.opts.Name)
	assert.Eq(t, "all", installer.opts.System)
	assert.True(t, installer.opts.DownloadOnly)
	assert.Eq(t, "eget", installer.opts.CacheName)
	assert.Eq(t, "1.7.1-18-gabcd1234", installer.opts.CacheVersion)
	assert.Eq(t, 0, len(installer.opts.Asset))
}

func TestSelfUpdateDownloadsToTempDir(t *testing.T) {
	replacement := filepath.Join(t.TempDir(), "eget")
	assert.NoErr(t, os.WriteFile(replacement, []byte("new"), 0o755))
	tempDir := filepath.Join(t.TempDir(), "self-update")
	installer := &fakeSelfUpdateInstaller{
		result: RunResult{ExtractedFiles: []string{replacement}},
	}
	svc := SelfUpdateService{
		CurrentVersion: "1.7.1",
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v1.7.2"}, nil
		},
		Installer:      installer,
		Replacer:       fakeSelfReplacer{},
		RuntimeGOOS:    "linux",
		RuntimeGOARCH:  "amd64",
		TempDir:        func() (string, error) { return tempDir, nil },
		ExecutablePath: func() (string, error) { return filepath.Join(t.TempDir(), "eget"), nil },
	}

	_, err := svc.Update(SelfUpdateOptions{
		Install: install.Options{Output: filepath.Join(t.TempDir(), "configured-output"), OutputExplicit: true},
	})

	assert.NoErr(t, err)
	assert.Eq(t, tempDir, installer.opts.Output)
	assert.False(t, installer.opts.OutputExplicit)
}

func TestSelfUpdateAssetFiltersDefaultToPlatformSelection(t *testing.T) {
	assert.Eq(t, 0, len(selfUpdateAssetFilters()))
}

func TestSelfUpdateDownloadNameUsesExeOnWindows(t *testing.T) {
	assert.Eq(t, "eget.exe", selfUpdateDownloadName("windows"))
}

func TestSelfUpdateDownloadsWindowsExecutableAsset(t *testing.T) {
	replacement := filepath.Join(t.TempDir(), "eget.exe")
	assert.NoErr(t, os.WriteFile(replacement, []byte("new"), 0o755))
	installer := &fakeSelfUpdateInstaller{
		result: RunResult{ExtractedFiles: []string{replacement}},
	}
	svc := SelfUpdateService{
		CurrentVersion: "1.7.1",
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v1.7.2"}, nil
		},
		Installer:      installer,
		Replacer:       fakeSelfReplacer{},
		RuntimeGOOS:    "windows",
		RuntimeGOARCH:  "amd64",
		ExecutablePath: func() (string, error) { return filepath.Join(t.TempDir(), "eget.exe"), nil },
	}

	_, err := svc.Update(SelfUpdateOptions{})

	assert.NoErr(t, err)
	assert.Eq(t, "windows/amd64", installer.opts.System)
	assert.Eq(t, "eget.exe", installer.opts.Name)
	assert.False(t, installer.opts.DownloadOnly)
	assert.Eq(t, "eget-windows-amd64.exe", installer.opts.ExtractFile)
}

func TestSelfUpdateReplacesExecutable(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "eget")
	replacement := filepath.Join(dir, "download", "eget")
	assert.NoErr(t, os.WriteFile(current, []byte("old"), 0o755))
	assert.NoErr(t, os.MkdirAll(filepath.Dir(replacement), 0o755))
	assert.NoErr(t, os.WriteFile(replacement, []byte("new"), 0o755))
	installer := &fakeSelfUpdateInstaller{
		result: RunResult{ExtractedFiles: []string{replacement}},
	}
	replacer := &recordingSelfReplacer{}
	svc := SelfUpdateService{
		CurrentVersion: "1.7.1",
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v1.7.2"}, nil
		},
		Installer:      installer,
		Replacer:       replacer,
		RuntimeGOOS:    "linux",
		RuntimeGOARCH:  "amd64",
		ExecutablePath: func() (string, error) { return current, nil },
	}

	result, err := svc.Update(SelfUpdateOptions{})

	assert.NoErr(t, err)
	assert.True(t, result.Updated)
	assert.Eq(t, current, result.Executable)
	assert.Eq(t, replacement, result.Replacement)
	assert.Eq(t, current, replacer.current)
	assert.Eq(t, replacement, replacer.replacement)
}

func TestSelfUpdateRejectsUnexpectedReplacementName(t *testing.T) {
	replacement := filepath.Join(t.TempDir(), "wrong")
	assert.NoErr(t, os.WriteFile(replacement, []byte("new"), 0o755))
	installer := &fakeSelfUpdateInstaller{
		result: RunResult{ExtractedFiles: []string{replacement}},
	}
	svc := SelfUpdateService{
		CurrentVersion: "1.7.1",
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v1.7.2"}, nil
		},
		Installer:     installer,
		Replacer:      fakeSelfReplacer{},
		RuntimeGOOS:   "linux",
		RuntimeGOARCH: "amd64",
	}

	_, err := svc.Update(SelfUpdateOptions{})

	assert.Err(t, err)
	assert.Contains(t, err.Error(), "replacement must be eget")
}

type fakeSelfUpdateInstaller struct {
	target string
	opts   install.Options
	result RunResult
}

func (f *fakeSelfUpdateInstaller) DownloadTarget(target string, opts install.Options) (RunResult, error) {
	f.target = target
	f.opts = opts
	return f.result, nil
}

type fakeSelfReplacer struct{}

func (fakeSelfReplacer) Replace(currentPath, replacementPath string) (SelfReplaceResult, error) {
	return SelfReplaceResult{}, nil
}

type recordingSelfReplacer struct {
	current     string
	replacement string
}

func (r *recordingSelfReplacer) Replace(currentPath, replacementPath string) (SelfReplaceResult, error) {
	r.current = currentPath
	r.replacement = replacementPath
	return SelfReplaceResult{}, nil
}
