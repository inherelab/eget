package web

import (
	"time"

	app "github.com/inherelab/eget/internal/app"
	"github.com/inherelab/eget/internal/extpkg"
)

// The console exposes its own DTOs so the HTTP contract stays stable even when
// the app layer structs change, and so the JSON is camelCase throughout.

type overviewResponse struct {
	Version      string      `json:"version"`
	ConfigPath   string      `json:"configPath"`
	ConfigExists bool        `json:"configExists"`
	Packages     int         `json:"packages"`
	Installed    int         `json:"installed"`
	Ext          []extBrief  `json:"ext"`
	Cache        *cacheBrief `json:"cache,omitempty"`
	Tasks        taskCounts  `json:"tasks"`
}

type cacheBrief struct {
	Dir   string `json:"dir"`
	Files int    `json:"files"`
	Size  int64  `json:"size"`
}

type taskCounts struct {
	Running int `json:"running"`
	Queued  int `json:"queued"`
}

type extBrief struct {
	Manager   string `json:"manager"`
	Available bool   `json:"available"`
	Bin       string `json:"bin,omitempty"`
}

type extManagerItem struct {
	Manager   string `json:"manager"`
	Available bool   `json:"available"`
	Bin       string `json:"bin,omitempty"`
	Packages  int    `json:"packages"`
}

type extManagersResponse struct {
	Managers []extManagerItem `json:"managers"`
	Failures []failureItem    `json:"failures,omitempty"`
}

type extPackagesResponse struct {
	Manager  string           `json:"manager"`
	Packages []extPackageItem `json:"packages"`
}

type extPackageItem struct {
	Manager string `json:"manager"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Latest  string `json:"latest,omitempty"`
}

type packageItem struct {
	Name         string     `json:"name"`
	Repo         string     `json:"repo"`
	Source       string     `json:"source"`
	Manager      string     `json:"manager,omitempty"`
	Target       string     `json:"target,omitempty"`
	Tag          string     `json:"tag,omitempty"`
	Version      string     `json:"version,omitempty"`
	Installed    bool       `json:"installed"`
	InstalledTag string     `json:"installedTag,omitempty"`
	InstalledAt  *time.Time `json:"installedAt,omitempty"`
	Asset        string     `json:"asset,omitempty"`
	AssetSize    int64      `json:"assetSize,omitempty"`
	URL          string     `json:"url,omitempty"`
	IsGUI        bool       `json:"isGui,omitempty"`
	InstallMode  string     `json:"installMode,omitempty"`
	IgnoreUpdate bool       `json:"ignoreUpdate,omitempty"`
}

type packageDetail struct {
	packageItem
	Desc           string         `json:"desc,omitempty"`
	Homepage       string         `json:"homepage,omitempty"`
	RepoURL        string         `json:"repoUrl,omitempty"`
	Configured     bool           `json:"configured"`
	ConfigTarget   string         `json:"configTarget,omitempty"`
	InstallTarget  string         `json:"installTarget,omitempty"`
	AssetURL       string         `json:"assetUrl,omitempty"`
	Tool           string         `json:"tool,omitempty"`
	ExtractedFiles []string       `json:"extractedFiles,omitempty"`
	Options        map[string]any `json:"options,omitempty"`
	UpdatedAt      *time.Time     `json:"updatedAt,omitempty"`
}

type packagesResponse struct {
	Total int           `json:"total"`
	Items []packageItem `json:"items"`
}

type outdatedItem struct {
	Name         string     `json:"name"`
	Repo         string     `json:"repo"`
	Source       string     `json:"source"`
	Manager      string     `json:"manager,omitempty"`
	Target       string     `json:"target,omitempty"`
	InstalledTag string     `json:"installedTag"`
	LatestTag    string     `json:"latestTag"`
	InstalledAt  *time.Time `json:"installedAt,omitempty"`
	PublishedAt  *time.Time `json:"publishedAt,omitempty"`
}

type outdatedResponse struct {
	Checked  int            `json:"checked"`
	Items    []outdatedItem `json:"items"`
	Failures []failureItem  `json:"failures,omitempty"`
}

type failureItem struct {
	Name  string `json:"name"`
	Repo  string `json:"repo,omitempty"`
	Error string `json:"error"`
}

type configView struct {
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Content string `json:"content"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func newPackageItem(item app.ListItem) packageItem {
	return packageItem{
		Name:         item.Name,
		Repo:         item.Repo,
		Source:       packageSource(item.Manager),
		Manager:      item.Manager,
		Target:       item.Target,
		Tag:          item.Tag,
		Version:      item.Version,
		Installed:    item.Installed,
		InstalledTag: item.InstalledTag,
		InstalledAt:  timePtr(item.InstalledAt),
		Asset:        item.Asset,
		AssetSize:    item.AssetSize,
		URL:          item.URL,
		IsGUI:        item.IsGUI,
		InstallMode:  item.InstallMode,
		IgnoreUpdate: item.IgnoreUpdate,
	}
}

func newPackageItems(items []app.ListItem) []packageItem {
	out := make([]packageItem, 0, len(items))
	for _, item := range items {
		out = append(out, newPackageItem(item))
	}
	return out
}

func newPackageDetail(res app.ShowResult) packageDetail {
	detail := packageDetail{
		packageItem: packageItem{
			Name:         res.Name,
			Repo:         res.Repo,
			Source:       packageSource(""),
			Tag:          res.Tag,
			Version:      res.Version,
			Installed:    res.Installed,
			InstalledTag: res.Tag,
			InstalledAt:  timePtr(res.InstalledAt),
			Asset:        res.Asset,
			IsGUI:        res.IsGUI,
			InstallMode:  res.InstallMode,
			URL:          res.AssetURL,
		},
		Desc:           res.Desc,
		Homepage:       res.Homepage,
		RepoURL:        res.RepoURL,
		Configured:     res.Configured,
		ConfigTarget:   res.ConfigTarget,
		InstallTarget:  res.InstallTarget,
		AssetURL:       res.AssetURL,
		Tool:           res.Tool,
		ExtractedFiles: res.ExtractedFiles,
		Options:        res.Options,
		UpdatedAt:      timePtr(res.UpdatedAt),
	}
	return detail
}

func newOutdatedItems(items []app.OutdatedItem) []outdatedItem {
	out := make([]outdatedItem, 0, len(items))
	for _, item := range items {
		out = append(out, outdatedItem{
			Name:         item.Name,
			Repo:         item.Repo,
			Source:       packageSource(item.Manager),
			Manager:      item.Manager,
			Target:       item.Target,
			InstalledTag: item.InstalledTag,
			LatestTag:    item.LatestTag,
			InstalledAt:  timePtr(item.InstalledAt),
			PublishedAt:  timePtr(item.PublishedAt),
		})
	}
	return out
}

func newFailureItems(failures []app.OutdatedCheckFailure) []failureItem {
	out := make([]failureItem, 0, len(failures))
	for _, failure := range failures {
		message := ""
		if failure.Error != nil {
			message = failure.Error.Error()
		}
		out = append(out, failureItem{Name: failure.Name, Repo: failure.Repo, Error: message})
	}
	return out
}

func newExtPackageItems(packages []extpkg.Package) []extPackageItem {
	out := make([]extPackageItem, 0, len(packages))
	for _, pkg := range packages {
		out = append(out, extPackageItem{
			Manager: pkg.Manager,
			Name:    pkg.Name,
			Version: pkg.Version,
			Latest:  pkg.Latest,
		})
	}
	return out
}

func packageSource(manager string) string {
	if manager != "" {
		return manager
	}
	return "eget"
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}
