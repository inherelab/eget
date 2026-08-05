package render

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gookit/cliui/show"
	"github.com/gookit/goutil/cliutil"
	"github.com/gookit/goutil/mathutil"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	"github.com/inherelab/eget/internal/sdk"
)

const CompactTimeLayout = "2006-01-02T15:04:05"
const compactDisplayTimeLayout = "2006-01-02 15:04:05"

type releaseDisplay struct {
	Tag         string `json:"tag,omitempty" mapstructure:"tag"`
	Name        string `json:"name,omitempty" mapstructure:"name"`
	PublishedAt string `json:"published_at,omitempty" mapstructure:"published_at"`
	Prerelease  bool   `json:"prerelease,omitempty" mapstructure:"prerelease"`
	AssetsCount int    `json:"assets_count,omitempty" mapstructure:"assets_count"`
}

type listItemDisplay struct {
	Name         string `mapstructure:"Name"`
	Repo         string `mapstructure:"Repo"`
	SourcePath   string `mapstructure:"SourcePath"`
	Target       string `mapstructure:"Target"`
	Tag          string `mapstructure:"Tag"`
	Version      string `mapstructure:"Version"`
	InstalledTag string `mapstructure:"InstalledTag"`
	Installed    bool   `mapstructure:"Installed"`
	InstalledAt  string `mapstructure:"InstalledAt"`
	Asset        string `mapstructure:"Asset"`
	URL          string `mapstructure:"URL"`
	IsGUI        bool   `mapstructure:"IsGUI"`
	InstallMode  string `mapstructure:"InstallMode"`
	IgnoreUpdate bool   `mapstructure:"IgnoreUpdate"`
}

type showResultDisplay struct {
	Name           string         `mapstructure:"Name"`
	Repo           string         `mapstructure:"Repo"`
	Description    string         `mapstructure:"Description"`
	Homepage       string         `mapstructure:"Homepage"`
	RepoURL        string         `mapstructure:"RepoURL"`
	Configured     bool           `mapstructure:"Configured"`
	Installed      bool           `mapstructure:"Installed"`
	ConfigTarget   string         `mapstructure:"ConfigTarget"`
	InstallTarget  string         `mapstructure:"InstallTarget"`
	Version        string         `mapstructure:"Version"`
	Tag            string         `mapstructure:"Tag"`
	InstalledAt    string         `mapstructure:"InstalledAt"`
	UpdatedAt      string         `mapstructure:"UpdatedAt"`
	Asset          string         `mapstructure:"Asset"`
	AssetURL       string         `mapstructure:"AssetURL"`
	Tool           string         `mapstructure:"Tool"`
	ExtractedFiles []string       `mapstructure:"ExtractedFiles"`
	IsGUI          bool           `mapstructure:"IsGUI"`
	InstallMode    string         `mapstructure:"InstallMode"`
	SourcePath     string         `mapstructure:"SourcePath"`
	Options        map[string]any `mapstructure:"Options"`
}

type repoInfoDisplay struct {
	Repo          string `json:"repo" mapstructure:"repo"`
	Description   string `json:"description,omitempty" mapstructure:"description"`
	Homepage      string `json:"homepage,omitempty" mapstructure:"homepage"`
	DefaultBranch string `json:"default_branch,omitempty" mapstructure:"default_branch"`
	Stars         int    `json:"stars,omitempty" mapstructure:"stars"`
	Forks         int    `json:"forks,omitempty" mapstructure:"forks"`
	OpenIssues    int    `json:"open_issues,omitempty" mapstructure:"open_issues"`
	UpdatedAt     string `json:"updated_at,omitempty" mapstructure:"updated_at"`
}

type assetDisplay struct {
	Name          string `json:"name" mapstructure:"name"`
	Size          int64  `json:"size,omitempty" mapstructure:"size"`
	DownloadCount int    `json:"download_count,omitempty" mapstructure:"download_count"`
	UpdatedAt     string `json:"updated_at,omitempty" mapstructure:"updated_at"`
	URL           string `json:"url,omitempty" mapstructure:"url"`
}

type urlInfoTextDisplay struct {
	RequestedURL string `mapstructure:"requested_url"`
	FinalURL     string `mapstructure:"final_url"`
	Status       string `mapstructure:"status"`
	FileName     string `mapstructure:"file_name"`
	Size         string `mapstructure:"size"`
	ContentType  string `mapstructure:"content_type"`
	LastModified string `mapstructure:"last_modified"`
	ETag         string `mapstructure:"etag"`
	AcceptRanges string `mapstructure:"accept_ranges"`
}

type queryResultDisplay struct {
	Action   string            `json:"action"`
	Repo     string            `json:"repo,omitempty"`
	Tag      string            `json:"tag,omitempty"`
	Info     *repoInfoDisplay  `json:"info,omitempty"`
	Latest   *releaseDisplay   `json:"latest,omitempty"`
	Releases []releaseDisplay  `json:"releases,omitempty"`
	Assets   []assetDisplay    `json:"assets,omitempty"`
	URLInfo  *app.QueryURLInfo `json:"url_info,omitempty"`
}

type searchRepoDisplay struct {
	FullName        string `json:"full_name,omitempty"`
	Description     string `json:"description,omitempty"`
	HTMLURL         string `json:"html_url,omitempty"`
	Homepage        string `json:"homepage,omitempty"`
	Language        string `json:"language,omitempty"`
	StargazersCount int    `json:"stargazers_count,omitempty"`
	ForksCount      int    `json:"forks_count,omitempty"`
	OpenIssuesCount int    `json:"open_issues_count,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
	Archived        bool   `json:"archived,omitempty"`
	Private         bool   `json:"private,omitempty"`
}

type searchResultDisplay struct {
	Query      string              `json:"query"`
	TotalCount int                 `json:"total_count"`
	Items      []searchRepoDisplay `json:"items,omitempty"`
}

type sdkInstalledEntryDisplay struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	Path            string `json:"path"`
	URL             string `json:"url,omitempty"`
	Filename        string `json:"filename,omitempty"`
	OS              string `json:"os,omitempty"`
	Arch            string `json:"arch,omitempty"`
	Ext             string `json:"ext,omitempty"`
	InstalledAt     string `json:"installed_at,omitempty"`
	StripComponents int    `json:"strip_components,omitempty"`
}

type sdkCachedIndexDisplay struct {
	SDK       string `json:"sdk"`
	Versions  int    `json:"versions,omitempty"`
	SourceURL string `json:"source_url,omitempty"`
	FetchedAt string `json:"fetched_at,omitempty"`
	Path      string `json:"path,omitempty"`
	Cached    bool   `json:"cached"`
}

type sdkIndexSummaryDisplay struct {
	SDK          string
	Source       string
	FetchedAt    string
	Versions     int
	Stable       int
	Files        int
	Latest       string
	LatestStable string
}

func CompactTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format(compactDisplayTimeLayout)
}

func CompactTimeOmit(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(CompactTimeLayout)
}

func TruncateTableText(value string, max int) string {
	if max <= 3 || len(value) <= max {
		return value
	}
	return value[:max-3] + "..."
}

func ListItemToDisplay(item app.ListItem) listItemDisplay {
	return listItemDisplay{
		Name:         item.Name,
		Repo:         item.Repo,
		SourcePath:   item.SourcePath,
		Target:       item.Target,
		Tag:          item.Tag,
		Version:      item.Version,
		InstalledTag: item.InstalledTag,
		Installed:    item.Installed,
		InstalledAt:  CompactTime(item.InstalledAt),
		Asset:        item.Asset,
		URL:          item.URL,
		IsGUI:        item.IsGUI,
		InstallMode:  item.InstallMode,
		IgnoreUpdate: item.IgnoreUpdate,
	}
}

func ShowResultToDisplay(result app.ShowResult) showResultDisplay {
	return showResultDisplay{
		Name:           result.Name,
		Repo:           result.Repo,
		Description:    result.Desc,
		Homepage:       result.Homepage,
		RepoURL:        result.RepoURL,
		Configured:     result.Configured,
		Installed:      result.Installed,
		ConfigTarget:   result.ConfigTarget,
		InstallTarget:  result.InstallTarget,
		Version:        result.Version,
		Tag:            result.Tag,
		InstalledAt:    CompactTime(result.InstalledAt),
		UpdatedAt:      CompactTime(result.UpdatedAt),
		Asset:          result.Asset,
		AssetURL:       result.AssetURL,
		Tool:           result.Tool,
		ExtractedFiles: append([]string(nil), result.ExtractedFiles...),
		IsGUI:          result.IsGUI,
		InstallMode:    result.InstallMode,
		SourcePath:     result.SourcePath,
		Options:        result.Options,
	}
}

func repoInfoToDisplay(info app.QueryRepoInfo) repoInfoDisplay {
	return repoInfoDisplay{
		Repo:          info.Repo,
		Description:   info.Description,
		Homepage:      info.Homepage,
		DefaultBranch: info.DefaultBranch,
		Stars:         info.Stars,
		Forks:         info.Forks,
		OpenIssues:    info.OpenIssues,
		UpdatedAt:     CompactTime(info.UpdatedAt),
	}
}

func releaseToDisplay(item app.QueryRelease) releaseDisplay {
	return releaseDisplay{
		Tag:         item.Tag,
		Name:        item.Name,
		PublishedAt: CompactTime(item.PublishedAt),
		Prerelease:  item.Prerelease,
		AssetsCount: item.AssetsCount,
	}
}

func releaseToJSONDisplay(item app.QueryRelease) releaseDisplay {
	display := releaseToDisplay(item)
	display.PublishedAt = CompactTimeOmit(item.PublishedAt)
	return display
}

func assetToDisplay(item app.QueryAsset) assetDisplay {
	return assetDisplay{
		Name:          item.Name,
		Size:          item.Size,
		DownloadCount: item.DownloadCount,
		UpdatedAt:     CompactTimeOmit(item.UpdatedAt),
		URL:           item.URL,
	}
}

func queryResultToDisplay(result app.QueryResult) queryResultDisplay {
	display := queryResultDisplay{
		Action: result.Action,
		Repo:   result.Repo,
		Tag:    result.Tag,
	}
	if result.Info != nil {
		info := repoInfoToDisplay(*result.Info)
		info.UpdatedAt = CompactTimeOmit(result.Info.UpdatedAt)
		display.Info = &info
	}
	if result.Latest != nil {
		latest := releaseToJSONDisplay(*result.Latest)
		display.Latest = &latest
	}
	for _, item := range result.Releases {
		display.Releases = append(display.Releases, releaseToJSONDisplay(item))
	}
	for _, item := range result.Assets {
		display.Assets = append(display.Assets, assetToDisplay(item))
	}
	display.URLInfo = result.URLInfo
	return display
}

func searchResultToDisplay(result app.SearchResult) searchResultDisplay {
	display := searchResultDisplay{
		Query:      result.Query,
		TotalCount: result.TotalCount,
		Items:      make([]searchRepoDisplay, 0, len(result.Items)),
	}
	for _, item := range result.Items {
		display.Items = append(display.Items, searchRepoDisplay{
			FullName:        item.FullName,
			Description:     item.Description,
			HTMLURL:         item.HTMLURL,
			Homepage:        item.Homepage,
			Language:        item.Language,
			StargazersCount: item.StargazersCount,
			ForksCount:      item.ForksCount,
			OpenIssuesCount: item.OpenIssuesCount,
			UpdatedAt:       CompactTimeOmit(item.UpdatedAt),
			Archived:        item.Archived,
			Private:         item.Private,
		})
	}
	return display
}

func QueryResultJSON(result app.QueryResult) (string, error) {
	data, err := json.MarshalIndent(queryResultToDisplay(result), "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func SearchResultJSON(result app.SearchResult) (string, error) {
	data, err := json.MarshalIndent(searchResultToDisplay(result), "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func SDKResultNotes(cached, resumed bool) string {
	var notes []string
	if cached {
		notes = append(notes, "cached")
	}
	if resumed {
		notes = append(notes, "resumed")
	}
	return strings.Join(notes, ", ")
}

func SDKEntriesToDisplay(entries []sdk.InstalledEntry) []sdkInstalledEntryDisplay {
	items := make([]sdkInstalledEntryDisplay, 0, len(entries))
	for _, entry := range entries {
		items = append(items, sdkInstalledEntryDisplay{
			Name:            entry.Name,
			Version:         entry.Version,
			Path:            entry.Path,
			URL:             entry.URL,
			Filename:        entry.Filename,
			OS:              entry.OS,
			Arch:            entry.Arch,
			Ext:             entry.Ext,
			InstalledAt:     CompactTimeOmit(entry.InstalledAt),
			StripComponents: entry.StripComponents,
		})
	}
	return items
}

func SDKCachedIndexesToDisplay(infos []sdk.CachedIndexInfo) []sdkCachedIndexDisplay {
	items := make([]sdkCachedIndexDisplay, 0, len(infos))
	for _, info := range infos {
		items = append(items, sdkCachedIndexDisplay{
			SDK:       info.SDK,
			Versions:  info.Versions,
			SourceURL: info.SourceURL,
			FetchedAt: CompactTimeOmit(info.FetchedAt),
			Path:      info.Path,
			Cached:    info.Cached,
		})
	}
	return items
}

func PrintSDKIndexSummary(index sdk.Index) {
	ccolor.Infoln("SDK Index:")
	summary := SDKIndexSummary(index)
	ccolor.Print(cliutil.FormatTable([]string{"Name", "Value"}, [][]any{
		{"SDK", summary.SDK},
		{"Source", summary.Source},
		{"Fetched At", summary.FetchedAt},
		{"Versions", summary.Versions},
		{"Stable", summary.Stable},
		{"Files", summary.Files},
		{"Latest", summary.Latest},
		{"Latest Stable", summary.LatestStable},
	}, cliutil.MinimalStyle))

	platforms := SDKIndexPlatformRows(index)
	if len(platforms) > 0 {
		ccolor.Print(cliutil.FormatTable([]string{"Platform", "Files"}, platforms, cliutil.MinimalStyle))
	}
}

func SDKIndexSummary(index sdk.Index) sdkIndexSummaryDisplay {
	stable := 0
	files := 0
	latest := ""
	latestStable := ""
	for _, item := range index.Items {
		files += len(item.Files)
		latest = item.Version
		if item.Stable {
			stable++
			latestStable = item.Version
		}
	}
	if latest == "" {
		latest = "-"
	}
	if latestStable == "" {
		latestStable = "-"
	}
	return sdkIndexSummaryDisplay{
		SDK:          index.SDK,
		Source:       index.SourceURL,
		FetchedAt:    CompactTime(index.FetchedAt),
		Versions:     len(index.Items),
		Stable:       stable,
		Files:        files,
		Latest:       latest,
		LatestStable: latestStable,
	}
}

func SDKIndexPlatformRows(index sdk.Index) [][]any {
	counts := map[string]int{}
	for _, item := range index.Items {
		for _, file := range item.Files {
			key := strings.Trim(file.OS+"/"+file.Arch+"."+file.Ext, "/.")
			if key == "" {
				key = "-"
			}
			counts[key]++
		}
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([][]any, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, []any{key, counts[key]})
	}
	return rows
}

func SDKIndexVersionRows(index sdk.Index, limit int) [][]any {
	if limit <= 0 || len(index.Items) == 0 {
		return nil
	}
	start := len(index.Items) - limit
	if start < 0 {
		start = 0
	}
	rows := make([][]any, 0, len(index.Items)-start)
	for i := len(index.Items) - 1; i >= start; i-- {
		item := index.Items[i]
		rows = append(rows, []any{item.Version, item.Stable, len(item.Files)})
	}
	return rows
}

func PrintJSON(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func PrintQueryResult(result app.QueryResult) {
	fmt.Printf("action: %s\n", result.Action)
	if result.URLInfo != nil {
		size := "unknown"
		if result.URLInfo.Size != nil && *result.URLInfo.Size >= 0 {
			size = mathutil.DataSize(uint64(*result.URLInfo.Size))
		}
		show.AList("URL Info", urlInfoTextDisplay{
			RequestedURL: result.URLInfo.RequestedURL,
			FinalURL:     result.URLInfo.FinalURL,
			Status:       result.URLInfo.Status,
			FileName:     result.URLInfo.FileName,
			Size:         size,
			ContentType:  result.URLInfo.ContentType,
			LastModified: result.URLInfo.LastModified,
			ETag:         result.URLInfo.ETag,
			AcceptRanges: result.URLInfo.AcceptRanges,
		})
		return
	}
	fmt.Printf("repo: %s\n", result.Repo)
	if result.Tag != "" {
		fmt.Printf("version: %s\n", result.Tag)
	}

	if result.Info != nil {
		info := repoInfoToDisplay(*result.Info)
		show.AList("Repo Info", info)
		return
	}

	if result.Latest != nil {
		latest := releaseToDisplay(*result.Latest)
		show.AList("Latest Release", latest)
		return
	}

	if len(result.Releases) > 0 {
		cols := []string{"Tag", "Name", "Published at", "Prerelease", "Assets Count"}
		rows := make([][]any, 0, len(result.Releases))
		for _, item := range result.Releases {
			rows = append(rows, []any{
				item.Tag,
				item.Name,
				CompactTime(item.PublishedAt),
				item.Prerelease,
				releaseAssetsCount(result.Repo, item),
			})
		}
		ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
		return
	}
	if len(result.Assets) > 0 {
		cols := []string{"Name", "Size", "Download Count"}
		rows := make([][]any, 0, len(result.Assets))
		for _, item := range result.Assets {
			rows = append(rows, []any{
				item.Name,
				mathutil.DataSize(uint64(item.Size)),
				item.DownloadCount,
			})
		}
		ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
	}
}

func releaseAssetsCount(repo string, item app.QueryRelease) any {
	if strings.HasPrefix(repo, "sourceforge:") && item.AssetsCount == 0 {
		return "-"
	}
	return item.AssetsCount
}

func PrintSearchResult(result app.SearchResult) {
	if len(result.Items) == 0 {
		ccolor.Infoln("no repositories found")
		return
	}

	for _, item := range result.Items {
		language := item.Language
		if language == "" {
			language = "-"
		}
		updatedAt := "-"
		if !item.UpdatedAt.IsZero() {
			updatedAt = CompactTime(item.UpdatedAt)
		}

		ccolor.Printf("<info>%s</> ⭐%d language: %s update: %s\n", item.FullName, item.StargazersCount, language, updatedAt)
		if item.Description != "" {
			ccolor.Printf("%s\n", item.Description)
		} else {
			ccolor.Println("No description")
		}
		fmt.Println("---")
	}
}
