package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
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
	IsGUI        bool
	InstallMode  string
	PurgedConfig string
}

type UninstallOptions struct {
	Purge bool
}

type uninstallTarget struct {
	Key  string
	Repo string
}

type UninstallService struct {
	Store      RemovableInstalledStore
	LoadConfig func() (*cfgpkg.File, error)
	SaveConfig func(*cfgpkg.File) error
}

func (s UninstallService) Uninstall(target string) (UninstallResult, error) {
	return s.UninstallWithOptions(target, UninstallOptions{})
}

func (s UninstallService) UninstallWithOptions(target string, opts UninstallOptions) (UninstallResult, error) {
	resolved, err := s.resolveTarget(target)
	if err != nil {
		return UninstallResult{}, err
	}
	if s.Store == nil {
		return UninstallResult{}, fmt.Errorf("installed store is required")
	}
	installedCfg, err := s.Store.Load()
	if err != nil {
		return UninstallResult{}, err
	}
	entry, key, ok := findUninstallEntry(installedCfg, resolved)
	if !ok {
		// A package can be configured without ever being installed (eget add,
		// or a failed install). With --purge, removing that config is the whole
		// job; without it there is nothing to remove.
		if !opts.Purge {
			return UninstallResult{}, fmt.Errorf("installed entry not found for %q", resolved.Key)
		}
		return s.purgeConfigOnly(resolved)
	}

	result := UninstallResult{
		Repo:        firstNonEmpty(entry.Repo, resolved.Repo),
		IsGUI:       entry.IsGUI,
		InstallMode: entry.InstallMode,
	}
	for _, file := range entry.ExtractedFiles {
		err := os.Remove(file)
		if err != nil && !os.IsNotExist(err) {
			return UninstallResult{}, err
		}
		result.RemovedFiles = append(result.RemovedFiles, file)
	}
	if err := removePortableGUIDirs(entry); err != nil {
		return UninstallResult{}, err
	}
	if err := s.Store.Remove(key); err != nil {
		return UninstallResult{}, err
	}
	if opts.Purge {
		configFile, err := s.loadConfig()
		if err != nil {
			return UninstallResult{}, err
		}
		purgedConfig, err := s.purgePackageConfig(configFile, resolved)
		if err != nil {
			return UninstallResult{}, err
		}
		result.PurgedConfig = purgedConfig
	}
	return result, nil
}

// purgeConfigOnly drops the [packages.<name>] section of a package that was
// never installed, so `eget rm --purge <name>` works for a config-only entry.
func (s UninstallService) purgeConfigOnly(target uninstallTarget) (UninstallResult, error) {
	configFile, err := s.loadConfig()
	if err != nil {
		return UninstallResult{}, err
	}
	purged, err := s.purgePackageConfig(configFile, target)
	if err != nil {
		return UninstallResult{}, err
	}
	if purged == "" {
		return UninstallResult{}, fmt.Errorf("%q is neither installed nor configured", target.Key)
	}
	return UninstallResult{Repo: target.Repo, PurgedConfig: purged}, nil
}

func removePortableGUIDirs(entry storepkg.Entry) error {
	if !entry.IsGUI || entry.InstallMode != install.InstallModePortable || len(entry.ExtractedFiles) == 0 {
		return nil
	}
	for _, file := range entry.ExtractedFiles {
		dir, err := filepath.Abs(filepath.Dir(file))
		if err != nil {
			return err
		}
		for _, root := range portableGUIUninstallRoots(entry) {
			rootPath, err := filepath.Abs(root.path)
			if err != nil {
				return err
			}
			if !pathWithinOrEqual(rootPath, dir) {
				continue
			}
			if err := removeEmptyDirsUpTo(dir, rootPath, root.keepRoot); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

type uninstallRoot struct {
	path     string
	keepRoot bool
}

func portableGUIUninstallRoots(entry storepkg.Entry) []uninstallRoot {
	roots := []uninstallRoot{}
	if output, _ := stringOption(entry.Options, "output", "target"); output != "" {
		roots = append(roots, uninstallRoot{path: output})
	}
	if guiTarget, _ := stringOption(entry.Options, "gui_target"); guiTarget != "" {
		roots = append(roots, uninstallRoot{path: guiTarget, keepRoot: true})
	}
	return roots
}

func removeEmptyDirsUpTo(dir, root string, keepRoot bool) error {
	for pathWithinOrEqual(root, dir) {
		if keepRoot && samePath(dir, root) {
			return nil
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if len(entries) > 0 {
			return nil
		}
		if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
			return err
		}
		if samePath(dir, root) {
			return nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
	return nil
}

func pathWithinOrEqual(root, path string) bool {
	if samePath(root, path) {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func (s UninstallService) resolveTarget(target string) (uninstallTarget, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return uninstallTarget{}, err
	}
	if pkg, ok := cfg.Packages[target]; ok {
		repo := util.DerefString(pkg.Repo)
		if repo == "" {
			return uninstallTarget{}, fmt.Errorf("package %q has no repo", target)
		}
		return uninstallTarget{Key: target, Repo: repo}, nil
	}
	if strings.Contains(target, "/") {
		return uninstallTarget{Key: target, Repo: target}, nil
	}
	return uninstallTarget{Key: target}, nil
}

func (s UninstallService) purgePackageConfig(cfg *cfgpkg.File, target uninstallTarget) (string, error) {
	if cfg == nil || len(cfg.Packages) == 0 {
		return "", nil
	}
	name, ok := purgePackageName(cfg, target)
	if !ok {
		return "", nil
	}
	delete(cfg.Packages, name)
	return name, s.saveConfig(cfg)
}

func purgePackageName(cfg *cfgpkg.File, target uninstallTarget) (string, bool) {
	if _, ok := cfg.Packages[target.Key]; ok {
		return target.Key, true
	}
	if target.Repo == "" {
		return "", false
	}
	match := ""
	normalizedRepo := storepkg.NormalizeRepoName(target.Repo)
	for name, pkg := range cfg.Packages {
		repo := util.DerefString(pkg.Repo)
		if repo == target.Repo || storepkg.NormalizeRepoName(repo) == normalizedRepo {
			if match != "" {
				return "", false
			}
			match = name
		}
	}
	return match, match != ""
}

func findUninstallEntry(cfg *storepkg.Config, target uninstallTarget) (storepkg.Entry, string, bool) {
	if cfg == nil || cfg.Installed == nil {
		return storepkg.Entry{}, "", false
	}
	for _, key := range uninstallCandidateKeys(target) {
		if entry, ok := cfg.Installed[key]; ok {
			return entry, key, true
		}
	}
	for key, entry := range cfg.Installed {
		if target.Key == repoName(entry.Repo) || target.Key == repoName(entry.Target) {
			return entry, key, true
		}
	}
	return storepkg.Entry{}, "", false
}

func uninstallCandidateKeys(target uninstallTarget) []string {
	keys := []string{target.Key}
	if normalized := storepkg.NormalizeRepoName(target.Key); normalized != target.Key {
		keys = append(keys, normalized)
	}
	if target.Repo != "" && target.Repo != target.Key {
		keys = append(keys, target.Repo)
	}
	if normalized := storepkg.NormalizeRepoName(target.Repo); normalized != "" && normalized != target.Repo {
		keys = append(keys, normalized)
	}
	return keys
}

func (s UninstallService) loadConfig() (*cfgpkg.File, error) {
	if s.LoadConfig != nil {
		return s.LoadConfig()
	}
	return cfgpkg.Load()
}

func (s UninstallService) saveConfig(cfg *cfgpkg.File) error {
	if s.SaveConfig != nil {
		return s.SaveConfig(cfg)
	}
	path, err := cfgpkg.ResolveWritablePath()
	if err != nil {
		return err
	}
	return cfgpkg.Save(path, cfg)
}
