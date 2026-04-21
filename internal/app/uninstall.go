package app

import (
	"fmt"
	"os"
	"strings"

	cfgpkg "github.com/inherelab/eget/internal/config"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

type RemovableInstalledStore interface {
	Load() (*storepkg.Config, error)
	Remove(target string) error
}

type UninstallResult struct {
	Repo         string
	RemovedFiles []string
}

type UninstallService struct {
	Store      RemovableInstalledStore
	LoadConfig func() (*cfgpkg.File, error)
}

func (s UninstallService) Uninstall(target string) (UninstallResult, error) {
	repo, err := s.resolveRepo(target)
	if err != nil {
		return UninstallResult{}, err
	}
	if s.Store == nil {
		return UninstallResult{}, fmt.Errorf("installed store is required")
	}
	cfg, err := s.Store.Load()
	if err != nil {
		return UninstallResult{}, err
	}
	entry, ok := cfg.Installed[repo]
	if !ok {
		return UninstallResult{}, fmt.Errorf("installed entry not found for %q", repo)
	}

	result := UninstallResult{Repo: repo}
	for _, file := range entry.ExtractedFiles {
		err := os.Remove(file)
		if err != nil && !os.IsNotExist(err) {
			return UninstallResult{}, err
		}
		result.RemovedFiles = append(result.RemovedFiles, file)
	}
	if err := s.Store.Remove(repo); err != nil {
		return UninstallResult{}, err
	}
	return result, nil
}

func (s UninstallService) resolveRepo(target string) (string, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return "", err
	}
	if pkg, ok := cfg.Packages[target]; ok {
		repo := util.DerefString(pkg.Repo)
		if repo == "" {
			return "", fmt.Errorf("package %q has no repo", target)
		}
		return repo, nil
	}
	if strings.Contains(target, "/") {
		return target, nil
	}
	return "", fmt.Errorf("unknown package %q", target)
}

func (s UninstallService) loadConfig() (*cfgpkg.File, error) {
	if s.LoadConfig != nil {
		return s.LoadConfig()
	}
	return cfgpkg.Load()
}
