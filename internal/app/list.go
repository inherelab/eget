package app

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

type InstalledLoader interface {
	Load() (*storepkg.Config, error)
}

type ListItem struct {
	Name           string
	Repo           string
	SourcePath     string
	Package        cfgpkg.Section
	Target         string
	Tag            string
	Version        string
	InstalledTag   string
	Installed      bool
	InstalledAt    time.Time
	Asset          string
	AssetID        int64
	AssetSize      int64
	AssetUpdatedAt time.Time
	AssetDigest    string
	URL            string
	IsGUI          bool
	InstallMode    string
	Prerelease     bool
	IgnoreUpdate   bool
}

type OutdatedItem struct {
	Name         string
	Repo         string
	Target       string
	InstalledTag string
	LatestTag    string
	InstalledAt  time.Time
	PublishedAt  time.Time
}

type OutdatedCheckFailure struct {
	Name  string
	Repo  string
	Error error
}

type LatestInfo struct {
	Tag            string
	PublishedAt    time.Time
	AssetID        int64
	AssetName      string
	AssetSize      int64
	AssetUpdatedAt time.Time
	AssetDigest    string
}

type LatestCheckTarget struct {
	Name       string
	Repo       string
	SourcePath string
	Package    cfgpkg.Section
	Tag        string
	Asset      string
	Prerelease bool
}

type LatestInfoFunc func(target LatestCheckTarget) (LatestInfo, error)

type ListService struct {
	LoadConfig    func() (*cfgpkg.File, error)
	LoadInstalled func() (*storepkg.Config, error)
	LatestInfo    LatestInfoFunc
	OnCheckDone   func(checked, total int)
}

func (s ListService) ListPackages() ([]ListItem, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, err
	}
	installed, err := s.loadInstalled()
	if err != nil {
		return nil, err
	}

	ignoredUpdates := ignoreUpdatePackageSet(cfg)
	byName := make(map[string]ListItem, len(cfg.Packages))
	for name, pkg := range cfg.Packages {
		repo := util.DerefString(pkg.Repo)
		item := ListItem{
			Name:         name,
			Repo:         repo,
			SourcePath:   util.DerefString(pkg.SourcePath),
			Package:      pkg,
			Target:       util.DerefString(pkg.Target),
			Tag:          util.DerefString(pkg.Tag),
			IgnoreUpdate: ignoredUpdates[name],
		}
		if pkg.Prerelease != nil && *pkg.Prerelease {
			item.Prerelease = true
		}
		if pkg.IsGUI != nil && *pkg.IsGUI {
			item.IsGUI = true
		}
		item = resolveListItemPackageTemplate(cfg, item)
		byName[name] = item
	}

	if installed != nil && installed.Installed != nil {
		repoToName := make(map[string]string, len(byName))
		for name, item := range byName {
			if item.Repo != "" {
				repoToName[item.Repo] = name
			}
		}
		for repo, entry := range installed.Installed {
			name := repoToName[repo]
			if name == "" {
				name = repoName(repo)
			}
			item, ok := byName[name]
			if !ok {
				item = ListItem{
					Name:         name,
					Repo:         firstNonEmpty(entry.Repo, repo),
					IgnoreUpdate: ignoredUpdates[name],
				}
			}
			if ignoredUpdates[name] {
				item.IgnoreUpdate = true
			}
			if item.Repo == "" {
				item.Repo = firstNonEmpty(entry.Repo, repo)
			}
			if item.Target == "" {
				item.Target = entry.Target
			}
			item.Installed = true
			item.Version = entry.Tag
			if item.Version == "" {
				item.Version = entry.Version
			}
			item.InstalledTag = entry.Tag
			item.InstalledAt = entry.InstalledAt
			item.Asset = entry.Asset
			item.AssetID = entry.AssetID
			item.AssetSize = entry.AssetSize
			item.AssetUpdatedAt = entry.AssetUpdatedAt
			item.AssetDigest = entry.AssetDigest
			item.URL = entry.URL
			if entry.IsGUI {
				item.IsGUI = true
			}
			if entry.InstallMode != "" {
				item.InstallMode = entry.InstallMode
			}
			if item.Tag == "" {
				if tag, ok := stringOption(entry.Options, "tag"); ok {
					item.Tag = tag
				}
			}
			if prerelease, ok := boolOption(entry.Options, "prerelease"); ok {
				item.Prerelease = prerelease
			}
			byName[name] = item
		}
	}

	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]ListItem, 0, len(names))
	for _, name := range names {
		items = append(items, byName[name])
	}
	return items, nil
}

func resolveListItemPackageTemplate(cfg *cfgpkg.File, item ListItem) ListItem {
	source, err := resolveInstallSourceSection(cfg, item.Repo)
	if err != nil {
		return item
	}
	if util.DerefString(source.URLTemplate) == "" && util.DerefString(source.LatestURL) == "" {
		return item
	}
	item.Package = latestCheckSectionFromSourceAndPackage(source, item.Package)
	return item
}

func latestCheckSectionFromSourceAndPackage(source, pkg cfgpkg.Section) cfgpkg.Section {
	merged := cfgpkg.MergeInstallOptions(cfgpkg.Section{}, source, pkg, cfgpkg.CLIOverrides{})
	section := pkg
	section.LatestURL = stringPtrIfNotEmpty(merged.LatestURL)
	section.LatestFormat = stringPtrIfNotEmpty(merged.LatestFormat)
	section.LatestJSONPath = stringPtrIfNotEmpty(merged.LatestJSONPath)
	section.VersionRegex = stringPtrIfNotEmpty(merged.VersionRegex)
	section.URLTemplate = stringPtrIfNotEmpty(merged.URLTemplate)
	section.OSMap = util.CloneStringMap(merged.OSMap)
	section.ArchMap = util.CloneStringMap(merged.ArchMap)
	section.ExtMap = util.CloneStringMap(merged.ExtMap)
	section.LibcMap = util.CloneStringMap(merged.LibcMap)
	section.ChecksumURLTemplate = stringPtrIfNotEmpty(merged.ChecksumURLTemplate)
	section.ChecksumFormat = stringPtrIfNotEmpty(merged.ChecksumFormat)
	section.ChecksumJSONPath = stringPtrIfNotEmpty(merged.ChecksumJSONPath)
	section.ChecksumRegex = stringPtrIfNotEmpty(merged.ChecksumRegex)
	section.InstallAction = stringPtrIfNotEmpty(merged.InstallAction)
	section.InstallArgs = append([]string(nil), merged.InstallArgs...)
	return section
}

func stringPtrIfNotEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return util.StringPtr(value)
}

func (s ListService) ListInstalledPackages() ([]ListItem, error) {
	items, err := s.ListPackages()
	if err != nil {
		return nil, err
	}
	installed := make([]ListItem, 0, len(items))
	for _, item := range items {
		if item.Installed {
			installed = append(installed, item)
		}
	}
	return installed, nil
}

func (s ListService) ListGUIPackages(all bool) ([]ListItem, error) {
	var items []ListItem
	var err error
	if all {
		items, err = s.ListPackages()
	} else {
		items, err = s.ListInstalledPackages()
	}
	if err != nil {
		return nil, err
	}
	gui := make([]ListItem, 0, len(items))
	for _, item := range items {
		if item.IsGUI {
			gui = append(gui, item)
		}
	}
	return gui, nil
}

func (s ListService) FindPackage(name string) (*ListItem, error) {
	items, err := s.ListPackages()
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Name == name {
			found := item
			return &found, nil
		}
	}
	return nil, fmt.Errorf("package %q not found", name)
}

func (s ListService) ListOutdatedPackages() ([]OutdatedItem, []OutdatedCheckFailure, int, error) {
	if s.LatestInfo == nil {
		return nil, nil, 0, fmt.Errorf("latest info checker is required")
	}

	cfg, err := s.loadConfig()
	if err != nil {
		return nil, nil, 0, err
	}
	listService := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
		LoadInstalled: s.loadInstalled,
		LatestInfo:    s.LatestInfo,
	}
	items, err := listService.ListPackages()
	if err != nil {
		return nil, nil, 0, err
	}

	outdated, failures, checked := checkOutdatedItems(items, s.LatestInfo, nil, batchConcurrencyFromConfig(cfg, install.Options{}), s.OnCheckDone)
	return outdated, failures, checked, nil
}

func checkOutdatedItems(items []ListItem, latestInfo LatestInfoFunc, include func(ListItem) bool, batch int, onCheckDone func(checked, total int)) ([]OutdatedItem, []OutdatedCheckFailure, int) {
	eligible := make([]ListItem, 0, len(items))
	for _, item := range items {
		if include != nil && !include(item) {
			continue
		}
		if !item.Installed || item.Repo == "" {
			continue
		}
		if item.IgnoreUpdate {
			continue
		}
		eligible = append(eligible, item)
	}
	if onCheckDone != nil {
		onCheckDone(0, len(eligible))
	}
	results := runOutdatedChecks(eligible, latestInfo, effectiveBatchConcurrency(batch, len(eligible)), onCheckDone)

	outdated := make([]OutdatedItem, 0, len(results))
	failures := make([]OutdatedCheckFailure, 0)
	for _, result := range results {
		if result.failure != nil {
			failures = append(failures, *result.failure)
		}
		if result.outdated != nil {
			outdated = append(outdated, *result.outdated)
		}
	}
	return outdated, failures, len(eligible)
}

func ignoreUpdatePackageSet(cfg *cfgpkg.File) map[string]bool {
	if cfg == nil || len(cfg.Global.IgnoreUpdatePackages) == 0 {
		return nil
	}
	ignored := make(map[string]bool, len(cfg.Global.IgnoreUpdatePackages))
	for _, name := range cfg.Global.IgnoreUpdatePackages {
		name = strings.TrimSpace(name)
		if name != "" {
			ignored[name] = true
		}
	}
	return ignored
}

type outdatedCheckResult struct {
	outdated *OutdatedItem
	failure  *OutdatedCheckFailure
}

func runOutdatedChecks(items []ListItem, latestInfo LatestInfoFunc, batch int, onCheckDone func(checked, total int)) []outdatedCheckResult {
	results := make([]outdatedCheckResult, len(items))
	if len(items) == 0 {
		return results
	}
	progress := newOutdatedCheckProgress(len(items), onCheckDone)
	if batch <= 1 {
		for i, item := range items {
			results[i] = checkOutdatedItem(item, latestInfo)
			progress.Done()
		}
		return results
	}

	jobs := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < batch; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				results[index] = checkOutdatedItem(items[index], latestInfo)
				progress.Done()
			}
		}()
	}
	for i := range items {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return results
}

type outdatedCheckProgress struct {
	total int
	done  int
	mu    sync.Mutex
	fn    func(checked, total int)
}

func newOutdatedCheckProgress(total int, fn func(checked, total int)) *outdatedCheckProgress {
	return &outdatedCheckProgress{total: total, fn: fn}
}

func (p *outdatedCheckProgress) Done() {
	if p == nil || p.fn == nil {
		return
	}
	p.mu.Lock()
	p.done++
	done := p.done
	total := p.total
	p.mu.Unlock()
	p.fn(done, total)
}

func checkOutdatedItem(item ListItem, latestInfo LatestInfoFunc) outdatedCheckResult {
	if item.InstalledTag == "" {
		failure := OutdatedCheckFailure{
			Name:  item.Name,
			Repo:  item.Repo,
			Error: fmt.Errorf("installed tag is empty"),
		}
		return outdatedCheckResult{failure: &failure}
	}

	latest, err := latestInfo(LatestCheckTarget{
		Name:       item.Name,
		Repo:       item.Repo,
		SourcePath: item.SourcePath,
		Package:    item.Package,
		Tag:        item.Tag,
		Asset:      item.Asset,
		Prerelease: item.Prerelease,
	})
	if err != nil {
		failure := OutdatedCheckFailure{
			Name:  item.Name,
			Repo:  item.Repo,
			Error: err,
		}
		return outdatedCheckResult{failure: &failure}
	}
	if latest.Tag == "" {
		return outdatedCheckResult{}
	}
	if latest.Tag == item.InstalledTag && !explicitTagAssetChanged(item, latest) {
		return outdatedCheckResult{}
	}

	outdated := OutdatedItem{
		Name:         item.Name,
		Repo:         item.Repo,
		Target:       item.Target,
		InstalledTag: item.InstalledTag,
		LatestTag:    latest.Tag,
		InstalledAt:  item.InstalledAt,
		PublishedAt:  latest.PublishedAt,
	}
	return outdatedCheckResult{outdated: &outdated}
}

func explicitTagAssetChanged(item ListItem, latest LatestInfo) bool {
	if item.Tag == "" || item.Asset == "" || latest.AssetName == "" || latest.AssetName != item.Asset {
		return false
	}
	if item.AssetDigest != "" && latest.AssetDigest != "" && item.AssetDigest != latest.AssetDigest {
		return true
	}
	if item.AssetID != 0 && latest.AssetID != 0 && item.AssetID != latest.AssetID {
		return true
	}
	if item.AssetSize != 0 && latest.AssetSize != 0 && item.AssetSize != latest.AssetSize {
		return true
	}
	if !item.AssetUpdatedAt.IsZero() && !latest.AssetUpdatedAt.IsZero() && !item.AssetUpdatedAt.Equal(latest.AssetUpdatedAt) {
		return true
	}
	return false
}

func repoName(repo string) string {
	parts := strings.Split(repo, "/")
	if len(parts) == 2 && parts[1] != "" {
		return parts[1]
	}
	return repo
}

func (s ListService) loadConfig() (*cfgpkg.File, error) {
	if s.LoadConfig != nil {
		return s.LoadConfig()
	}
	return cfgpkg.Load()
}

func (s ListService) loadInstalled() (*storepkg.Config, error) {
	if s.LoadInstalled != nil {
		return s.LoadInstalled()
	}
	store, err := storepkg.DefaultStore()
	if err != nil {
		return nil, err
	}
	return store.Load()
}
