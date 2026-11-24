package app

import (
	"fmt"
	"time"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
)

const (
	maxChunkConcurrency         = 32
	maxBatchConcurrency         = 16
	defaultAutoBatchConcurrency = 6
)

type RunResult = install.RunResult

type Runner interface {
	Run(target string, opts install.Options) (RunResult, error)
}

type InstalledStore interface {
	Record(target string, entry storepkg.Entry) error
}

type PackageAdder interface {
	AddPackage(repo, name string, opts install.Options) error
}

type InstallExtras struct {
	AddToConfig bool
	PackageName string
	PackageOpts install.Options
}

type InstallAllResult struct {
	Name   string
	Target string
	Result RunResult
}

// ReleaseInfoFunc resolves the tag and publish time of a release, honouring the
// prerelease option of the install.
type ReleaseInfoFunc func(repo, url string, includePrerelease bool) (string, time.Time, error)

type Service struct {
	Runner       Runner
	Store        InstalledStore
	Config       PackageAdder
	Now          func() time.Time
	ReleaseInfo  ReleaseInfoFunc
	RepoMetadata func(repo string) (RepoMetadata, error)
	LoadConfig   func() (*cfgpkg.File, error)
}

func (s Service) InstallTarget(target string, opts install.Options, extras ...InstallExtras) (RunResult, error) {
	if err := validateRawConcurrencyOptions(opts); err != nil {
		return RunResult{}, err
	}
	opts, extras = inferAddPackageName(target, opts, extras)
	runTarget, recordTarget, opts, err := s.resolveInstallRequest(target, opts, false)
	if err != nil {
		return RunResult{}, err
	}
	if len(extras) > 0 && extras[0].AddToConfig && extras[0].PackageName == "" {
		extras[0].PackageName = recordTarget
		extras[0].PackageOpts.Name = recordTarget
	}
	opts = applyDefaultInstallTarget(opts)
	if err := validateConcurrencyOptions(opts); err != nil {
		return RunResult{}, err
	}
	opts = normalizeExtractionOptions(opts)
	result, err := s.installResolvedTarget(runTarget, recordTarget, opts)
	if err != nil {
		return RunResult{}, err
	}

	if len(extras) > 0 && extras[0].AddToConfig {
		if s.Config == nil {
			return RunResult{}, fmt.Errorf("config service is required")
		}
		repo := runTarget
		if normalized, err := install.NormalizeRepoTarget(runTarget); err == nil {
			repo = normalized
		} else if !isManagedConfigTarget(runTarget) {
			return RunResult{}, err
		}
		addOpts := extras[0].PackageOpts
		if result.IsGUI {
			addOpts.IsGUI = true
		}
		if err := s.Config.AddPackage(repo, extras[0].PackageName, addOpts); err != nil {
			return RunResult{}, err
		}
	}

	return result, nil
}

func (s Service) DownloadTarget(target string, opts install.Options) (RunResult, error) {
	if err := validateRawConcurrencyOptions(opts); err != nil {
		return RunResult{}, err
	}
	runTarget, _, opts, err := s.resolveInstallRequest(target, opts, true)
	if err != nil {
		return RunResult{}, err
	}
	if err := validateConcurrencyOptions(opts); err != nil {
		return RunResult{}, err
	}
	opts = normalizeExtractionOptions(opts)
	opts.DownloadOnly = opts.ExtractFile == "" && !opts.All
	return s.Runner.Run(runTarget, opts)
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s Service) loadConfig() (*cfgpkg.File, error) {
	if s.LoadConfig != nil {
		return s.LoadConfig()
	}
	return cfgpkg.Load()
}
