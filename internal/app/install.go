package app

import (
	"path"
	"time"

	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
)

type RunResult = install.RunResult

type Runner interface {
	Run(target string, opts install.Options) (RunResult, error)
}

type InstalledStore interface {
	Record(target string, entry storepkg.Entry) error
}

type ReleaseInfoFunc func(repo, url string) (string, time.Time, error)

type Service struct {
	Runner      Runner
	Store       InstalledStore
	Now         func() time.Time
	ReleaseInfo ReleaseInfoFunc
}

func (s Service) InstallTarget(target string, opts install.Options) (RunResult, error) {
	result, err := s.Runner.Run(target, opts)
	if err != nil {
		return RunResult{}, err
	}

	if s.Store != nil && len(result.ExtractedFiles) > 0 {
		repo := storepkg.NormalizeRepoName(target)
		tag, releaseDate := "", time.Time{}
		if s.ReleaseInfo != nil {
			if gotTag, gotDate, err := s.ReleaseInfo(repo, result.URL); err == nil {
				tag = gotTag
				releaseDate = gotDate
			}
		}

		entry := storepkg.Entry{
			Repo:           repo,
			Target:         target,
			InstalledAt:    s.now(),
			URL:            result.URL,
			Asset:          chooseAsset(result),
			Tool:           result.Tool,
			ExtractedFiles: append([]string(nil), result.ExtractedFiles...),
			Options:        extractOptionsMap(opts),
			Tag:            tag,
			ReleaseDate:    releaseDate,
		}
		if err := s.Store.Record(target, entry); err != nil {
			return RunResult{}, err
		}
	}

	return result, nil
}

func (s Service) DownloadTarget(target string, opts install.Options) (RunResult, error) {
	opts.DownloadOnly = true
	return s.Runner.Run(target, opts)
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func chooseAsset(result RunResult) string {
	if result.Asset != "" {
		return result.Asset
	}
	return path.Base(result.URL)
}

func extractOptionsMap(opts install.Options) map[string]interface{} {
	recorded := make(map[string]interface{})
	if opts.Tag != "" {
		recorded["tag"] = opts.Tag
	}
	if opts.System != "" {
		recorded["system"] = opts.System
	}
	if opts.Output != "" {
		recorded["output"] = opts.Output
	}
	if opts.ExtractFile != "" {
		recorded["extract_file"] = opts.ExtractFile
	}
	if opts.All {
		recorded["all"] = true
	}
	if opts.Quiet {
		recorded["quiet"] = true
	}
	if opts.DownloadOnly {
		recorded["download_only"] = true
	}
	if opts.UpgradeOnly {
		recorded["upgrade_only"] = true
	}
	if len(opts.Asset) > 0 {
		recorded["asset"] = append([]string(nil), opts.Asset...)
	}
	if opts.Hash {
		recorded["hash"] = true
	}
	if opts.Verify != "" {
		recorded["verify"] = opts.Verify
	}
	if opts.Source {
		recorded["download_source"] = true
	}
	if opts.DisableSSL {
		recorded["disable_ssl"] = true
	}
	return recorded
}
