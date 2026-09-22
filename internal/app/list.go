package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/extpkg"
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
	// Manager is the external package manager that owns this package, empty for
	// packages eget installed itself.
	Manager string
}

type OutdatedItem struct {
	Name         string
	Repo         string
	Target       string
	InstalledTag string
	LatestTag    string
	InstalledAt  time.Time
	PublishedAt  time.Time
	// Manager is set for packages owned by an external package manager.
	Manager string
}

// Managers modes decide which external packages take part in list and update.
// The zero value (or "off") keeps eget's behavior unchanged.
const (
	ManagersModeOff  = "off"
	ManagersModeWith = "with"
	ManagersModeOnly = "only"
)

// ManagersSelection is the resolved choice of which external managers take part
// in list/update. It is produced by the CLI from its flags and [global]
// managers_mode, so config can only yield off and with.
type ManagersSelection struct {
	Mode string
	// Managers restricts the selection to these manager names. Empty while Mode
	// is not off means "every configured manager".
	Managers []string
}

// Enabled reports whether any external manager takes part.
func (s ManagersSelection) Enabled() bool {
	return s.Mode == ManagersModeWith || s.Mode == ManagersModeOnly
}

// OnlyManagers reports whether eget's own packages are excluded.
func (s ManagersSelection) OnlyManagers() bool { return s.Mode == ManagersModeOnly }

// ExternalProvider lists and upgrades packages owned by external managers. It
// is implemented by extpkg.Service; the manager name list filters the work so a
// single-manager selection does not run every manager.
type ExternalProvider interface {
	List(ctx context.Context, only ...string) ([]extpkg.Package, []extpkg.Failure, error)
	Outdated(ctx context.Context, only ...string) ([]extpkg.Package, []extpkg.Failure, error)
	Upgrade(ctx context.Context, managerName string, names []string) (extpkg.UpgradeResult, error)
	Manager(name string) (extpkg.Manager, bool)
	Names() []string
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
	// External and Managers add packages owned by external managers. Both are
	// optional: with a zero ManagersSelection nothing external runs at all.
	External ExternalProvider
	Managers ManagersSelection
	// OnExternalFailure reports a manager-level failure while listing. It is a
	// callback because list has no failure channel of its own.
	OnExternalFailure func(extpkg.Failure)
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
				tag := entry.Tag
				if optionTag, ok := stringOption(entry.Options, "tag"); ok {
					tag = optionTag
				}
				item.Tag = trackingTagWithPolicy(tag, installedTagPolicy(entry))
			} else {
				item.Tag = trackingTagWithPolicy(item.Tag, itemTagPolicy(item, entry))
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

	external := s.externalItems(ignoredUpdates)
	if s.Managers.OnlyManagers() {
		return external, nil
	}
	if len(external) == 0 {
		return items, nil
	}
	items = append(items, external...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].Manager < items[j].Manager
	})
	return items, nil
}

// externalItems lists the packages of the selected external managers. With a
// zero ManagersSelection nothing is run and no process is started.
func (s ListService) externalItems(ignored map[string]bool) []ListItem {
	if !s.Managers.Enabled() || s.External == nil {
		return nil
	}
	packages, failures, err := s.External.List(context.Background(), s.Managers.Managers...)
	for _, failure := range failures {
		if s.OnExternalFailure != nil {
			s.OnExternalFailure(failure)
		}
	}
	if err != nil {
		if s.OnExternalFailure != nil {
			s.OnExternalFailure(extpkg.Failure{Err: err})
		}
		return nil
	}

	items := make([]ListItem, 0, len(packages))
	for _, pkg := range packages {
		ref := pkg.Manager + ":" + pkg.Name
		items = append(items, ListItem{
			Name:         pkg.Name,
			Repo:         ref,
			Manager:      pkg.Manager,
			Version:      pkg.Version,
			InstalledTag: pkg.Version,
			Installed:    true,
			IgnoreUpdate: ignored[pkg.Name] || ignored[ref],
		})
	}
	return items
}

// externalOutdatedItems collects the outdated packages of the selected external
// managers, honouring ignore_update_packages the same way the repo checks do.
func (s ListService) externalOutdatedItems(ignored map[string]bool) ([]OutdatedItem, []OutdatedCheckFailure, int) {
	if !s.Managers.Enabled() || s.External == nil {
		return nil, nil, 0
	}
	packages, failures, err := s.External.Outdated(context.Background(), s.Managers.Managers...)
	if err != nil {
		return nil, []OutdatedCheckFailure{{Name: "managers", Error: err}}, 0
	}

	outdated := make([]OutdatedItem, 0, len(packages))
	for _, pkg := range packages {
		if ignoredExternalPackage(ignored, pkg) {
			continue
		}
		outdated = append(outdated, OutdatedItem{
			Name:         pkg.Name,
			Repo:         pkg.Manager + ":" + pkg.Name,
			InstalledTag: pkg.Version,
			LatestTag:    pkg.Latest,
			Manager:      pkg.Manager,
		})
	}

	// A manager-level failure names the manager in both fields so the existing
	// "check_failed %s (%s)" output does not render empty parentheses.
	checkFailures := make([]OutdatedCheckFailure, 0, len(failures))
	for _, failure := range failures {
		checkFailures = append(checkFailures, OutdatedCheckFailure{
			Name:  failure.Manager,
			Repo:  failure.Manager,
			Error: failure.Err,
		})
	}
	// One manager command checks every package it owns at once, so the checked
	// count is its installed set and not the outdated one.
	return outdated, checkFailures, s.checkedExternalPackages(ignored)
}

// checkedExternalPackages counts the packages covered by the manager commands:
// everything installed by the selected managers except the ignored ones, which
// matches how the repo checks count their eligible items.
func (s ListService) checkedExternalPackages(ignored map[string]bool) int {
	installed, _, err := s.External.List(context.Background(), s.Managers.Managers...)
	if err != nil {
		return 0
	}
	checked := 0
	for _, pkg := range installed {
		if !ignoredExternalPackage(ignored, pkg) {
			checked++
		}
	}
	return checked
}

func ignoredExternalPackage(ignored map[string]bool, pkg extpkg.Package) bool {
	ref := pkg.Manager + ":" + pkg.Name
	return ignored[pkg.Name] || ignored[ref]
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

	var outdated []OutdatedItem
	var failures []OutdatedCheckFailure
	checked := 0
	if !s.Managers.OnlyManagers() {
		batch := batchConcurrencyFromConfig(cfg, install.Options{})
		outdated, failures, checked = checkOutdatedItems(items, s.LatestInfo, nil, batch, s.OnCheckDone)
	}

	// External packages are checked by their own manager, in one batched call
	// per selected manager rather than one check per package.
	externalOutdated, externalFailures, externalChecked := s.externalOutdatedItems(ignoreUpdatePackageSet(cfg))
	outdated = append(outdated, externalOutdated...)
	failures = append(failures, externalFailures...)
	checked += externalChecked

	sort.SliceStable(outdated, func(i, j int) bool {
		if outdated[i].Name != outdated[j].Name {
			return outdated[i].Name < outdated[j].Name
		}
		return outdated[i].Manager < outdated[j].Manager
	})
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
		// External packages are checked by their own manager, never through
		// LatestInfoFunc (which would treat "npm:x" as a GitHub repo).
		if item.Manager != "" {
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
