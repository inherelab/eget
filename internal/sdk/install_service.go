package sdk

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/inherelab/eget/internal/install"
)

func (s Service) Install(ctx context.Context, rawTarget string, opts InstallOptions) (InstallResult, error) {
	archive, err := s.resolveDownloadArchive(rawTarget, PlatformOptions{})
	if err != nil {
		return InstallResult{}, err
	}
	cfg := archive.Config
	vars := TemplateVars{Name: cfg.Name, Version: archive.Version, OS: archive.OS, Arch: archive.Arch, Ext: archive.Ext}
	installPath, err := s.resolveInstallPath(cfg, vars)
	if err != nil {
		return InstallResult{}, err
	}
	if _, err := os.Stat(installPath); err == nil {
		if !opts.Force {
			return InstallResult{}, fmt.Errorf("sdk install path already exists: %s", installPath)
		}
		if err := s.ensureSafeSDKPath(installPath, cfg); err != nil {
			return InstallResult{}, err
		}
		if err := os.RemoveAll(installPath); err != nil {
			return InstallResult{}, err
		}
	} else if !os.IsNotExist(err) {
		return InstallResult{}, err
	}

	if opts.OnStart != nil {
		host := indexSourceHost(archive.URL)
		if host == "" {
			host = archive.URL
		}
		opts.OnStart(rawTarget, archive.Version, host)
	}
	download := s.Downloader
	if download == nil {
		download = DownloadArchive
	}
	downloadResult, err := download(ctx, DownloadRequest{
		URL:         archive.URL,
		CacheDir:    s.cacheDir(),
		SDK:         cfg.Name,
		Version:     archive.Version,
		Filename:    archive.Filename,
		ClientOpts:  s.effectiveClientOptions(),
		CacheMirror: s.CacheMirror,
		Progress:    opts.Progress,
	})
	if err != nil {
		return InstallResult{}, err
	}
	if opts.OnArchiveReady != nil {
		opts.OnArchiveReady(downloadResult)
	}

	tmpDir := filepath.Join(s.sdkRoot(cfg), ".eget-tmp", fmt.Sprintf("%s-%s-%d", cfg.Name, archive.Version, s.now().UnixNano()))
	if err := os.RemoveAll(tmpDir); err != nil {
		return InstallResult{}, err
	}
	defer os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return InstallResult{}, err
	}
	chooser, err := install.NewGlobChooser("*")
	if err != nil {
		return InstallResult{}, err
	}
	extractor := install.NewExtractor(archive.Filename, cfg.Name, chooser)
	direct, ok := extractor.(install.DirectAllExtractor)
	if !ok {
		return InstallResult{}, fmt.Errorf("sdk archive extractor does not support direct extraction")
	}
	source, err := os.Open(downloadResult.Path)
	if err != nil {
		return InstallResult{}, err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return InstallResult{}, err
	}
	if opts.OnExtractStart != nil {
		opts.OnExtractStart(downloadResult.Path, installPath)
	}
	if withOptions, ok := direct.(interface {
		ExtractAllToWithOptions(io.ReadSeeker, int64, string, install.ArchiveExtractOptions) ([]string, error)
	}); ok {
		if _, err := withOptions.ExtractAllToWithOptions(source, info.Size(), tmpDir, install.ArchiveExtractOptions{StripComponents: cfg.StripComponents}); err != nil {
			return InstallResult{}, err
		}
	} else {
		if _, err := direct.ExtractAllTo(source, info.Size(), tmpDir); err != nil {
			return InstallResult{}, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(installPath), 0o755); err != nil {
		return InstallResult{}, err
	}
	if err := os.Rename(tmpDir, installPath); err != nil {
		return InstallResult{}, err
	}

	entry := InstalledEntry{
		Name:            cfg.Name,
		Version:         archive.Version,
		Path:            installPath,
		URL:             archive.URL,
		Filename:        archive.Filename,
		OS:              archive.OS,
		Arch:            archive.Arch,
		Ext:             archive.Ext,
		InstalledAt:     s.now(),
		StripComponents: cfg.StripComponents,
	}
	if err := s.Store.Record(entry); err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Name: cfg.Name, Version: archive.Version, Path: installPath, URL: archive.URL, Cached: downloadResult.FromCache, Resumed: downloadResult.Resumed}, nil
}

func (s Service) InstallMany(ctx context.Context, targets []string, opts InstallOptions) ([]InstallResult, error) {
	results := make([]InstallResult, 0, len(targets))
	for _, target := range targets {
		result, err := s.Install(ctx, target, opts)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

type resolvedArchive struct {
	Config   Config
	Version  string
	URL      string
	Filename string
	OS       string
	Arch     string
	Ext      string
}

func (s Service) resolveDownloadArchive(rawTarget string, platform PlatformOptions) (resolvedArchive, error) {
	target, err := ParseTarget(rawTarget)
	if err != nil {
		return resolvedArchive{}, err
	}
	cfg, err := s.resolveConfigForPlatform(target.Name, platform)
	if err != nil {
		return resolvedArchive{}, err
	}
	version, file, err := s.resolveVersionAndFile(target, cfg)
	if err != nil {
		return resolvedArchive{}, err
	}
	vars := TemplateVars{Name: cfg.Name, Version: version, OS: cfg.OS, Arch: cfg.Arch, Ext: cfg.Ext}
	url := file.URL
	filename := file.Filename
	if url == "" {
		url, err = RenderTemplate(cfg.URLTemplate, vars)
		if err != nil {
			return resolvedArchive{}, err
		}
		filename = filepath.Base(url)
	}
	if filename == "" {
		filename = filepath.Base(url)
	}
	return resolvedArchive{
		Config:   cfg,
		Version:  version,
		URL:      url,
		Filename: filename,
		OS:       cfg.OS,
		Arch:     cfg.Arch,
		Ext:      cfg.Ext,
	}, nil
}

func (s Service) ensureSafeSDKPath(targetPath string, cfg Config) error {
	root, err := filepath.Abs(s.sdkRoot(cfg))
	if err != nil {
		return err
	}
	target, err := filepath.Abs(targetPath)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if rel == "." || rel == "" || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." || filepath.IsAbs(rel) {
		return fmt.Errorf("unsafe sdk path %q outside %q", targetPath, root)
	}
	return nil
}
