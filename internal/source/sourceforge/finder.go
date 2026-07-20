package sourceforge

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
)

type HTTPGetter interface {
	Get(url string) (*http.Response, error)
}

type Finder struct {
	Project string
	Path    string
	Tag     string
	Getter  HTTPGetter
}

type LatestInfo struct {
	Tag         string
	Version     string
	Path        string
	PublishedAt time.Time
	Prerelease  bool
	AssetsCount int
}

func (f Finder) Find() ([]string, error) {
	if strings.TrimSpace(f.Project) == "" {
		return nil, fmt.Errorf("sourceforge project is required")
	}
	if f.Getter == nil {
		return nil, fmt.Errorf("sourceforge HTTP getter is required")
	}

	sourcePath := strings.Trim(strings.Trim(f.Path, "/")+"/"+strings.Trim(f.Tag, "/"), "/")
	files, err := f.list(sourcePath)
	if err != nil {
		return nil, err
	}

	prioritizeNewestFiles(files)
	urls := downloadableURLs(files)
	if len(urls) > 0 {
		return urls, nil
	}

	latest, ok := LatestVersionFile(files)
	if !ok {
		if sourcePath == "" {
			stable, stableOK := stableDirectory(files)
			if stableOK {
				files, err = f.list(stable.FullPath)
				if err != nil {
					return nil, err
				}
				latest, ok = LatestVersionFile(files)
			}
		}
	}
	if !ok {
		return nil, fmt.Errorf("sourceforge downloadable files not found")
	}
	files, err = f.list(latest.FullPath)
	if err != nil {
		return nil, err
	}

	prioritizeNewestFiles(files)
	urls = downloadableURLs(files)
	if len(urls) == 0 {
		return nil, fmt.Errorf("sourceforge downloadable files not found")
	}
	return urls, nil
}

func (f Finder) FallbackVersionAssets(limit int) ([][]string, error) {
	if limit <= 0 {
		return nil, nil
	}
	if strings.TrimSpace(f.Project) == "" {
		return nil, fmt.Errorf("sourceforge project is required")
	}
	if f.Getter == nil {
		return nil, fmt.Errorf("sourceforge HTTP getter is required")
	}

	files, err := f.list(strings.Trim(f.Path, "/"))
	if err != nil {
		return nil, err
	}
	versions := sortedVersionDirectories(files)
	if len(versions) <= 1 {
		return nil, nil
	}

	var groups [][]string
	attempts := 0
	for _, version := range versions[1:] {
		if attempts >= limit {
			break
		}
		attempts++
		files, err := f.list(version.FullPath)
		if err != nil {
			return nil, err
		}
		urls := downloadableURLs(files)
		if len(urls) > 0 {
			groups = append(groups, urls)
		}
	}
	return groups, nil
}

func LatestVersion(project, sourcePath string, getter HTTPGetter) (LatestInfo, error) {
	finder := Finder{Project: project, Path: sourcePath, Getter: getter}
	files, err := finder.list(strings.Trim(sourcePath, "/"))
	if err != nil {
		return LatestInfo{}, err
	}

	versions, err := releaseVersionFiles(finder, files, sourcePath)
	if err != nil {
		return LatestInfo{}, err
	}
	if len(versions) == 0 {
		if info, ok := latestDownloadableFileInfo(files); ok {
			return info, nil
		}
		return LatestInfo{}, sourceForgeLatestNotFoundError(project, sourcePath, files)
	}
	return releaseInfo(finder, versions[0])
}

func ListReleases(project, sourcePath string, limit int, includePrerelease bool, getter HTTPGetter) ([]LatestInfo, error) {
	if limit <= 0 {
		limit = 10
	}
	finder := Finder{Project: project, Path: sourcePath, Getter: getter}
	files, err := finder.list(strings.Trim(sourcePath, "/"))
	if err != nil {
		return nil, err
	}

	versions, err := releaseVersionFiles(finder, files, sourcePath)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("could not determine SourceForge releases for %s", project)
	}

	releases := make([]LatestInfo, 0, min(limit, len(versions)))
	for _, version := range versions {
		if len(releases) == limit {
			break
		}
		if !includePrerelease && isPrereleaseVersion(version) {
			continue
		}
		releases = append(releases, releaseInfoFromVersionFile(version))
	}
	return releases, nil
}

func releaseVersionFiles(finder Finder, files []File, sourcePath string) ([]File, error) {
	versions := sortedVersionDirectories(files)
	if len(versions) == 0 && sourcePath == "" {
		stable, stableOK := stableDirectory(files)
		if stableOK {
			var err error
			files, err = finder.list(stable.FullPath)
			if err != nil {
				return nil, err
			}
			versions = sortedVersionDirectories(files)
		}
	}
	return versions, nil
}

func releaseInfo(finder Finder, versionFile File) (LatestInfo, error) {
	version := fileVersion(versionFile)
	if version == "" {
		return LatestInfo{}, fmt.Errorf("could not determine SourceForge version from %s", versionFile.Name)
	}
	assets, err := finder.list(versionFile.FullPath)
	if err != nil {
		return LatestInfo{}, err
	}
	return LatestInfo{
		Tag:         sourceForgeReleaseTag(versionFile),
		Version:     version,
		Path:        versionFile.FullPath,
		PublishedAt: latestPublishedAt(assets),
		Prerelease:  isPrereleaseVersion(versionFile),
		AssetsCount: len(downloadableURLs(assets)),
	}, nil
}

func releaseInfoFromVersionFile(versionFile File) LatestInfo {
	return LatestInfo{
		Tag:         sourceForgeReleaseTag(versionFile),
		Version:     fileVersion(versionFile),
		Path:        versionFile.FullPath,
		PublishedAt: versionFile.PublishedAt,
		Prerelease:  isPrereleaseVersion(versionFile),
	}
}

func latestDownloadableFileInfo(files []File) (LatestInfo, bool) {
	var latest File
	count := 0
	for _, file := range files {
		if file.Type != TypeFile || file.DownloadURL == "" {
			continue
		}
		count++
		if latest.DownloadURL == "" || file.PublishedAt.After(latest.PublishedAt) {
			latest = file
		}
	}
	if latest.DownloadURL == "" {
		return LatestInfo{}, false
	}
	name := sourceForgeReleaseTag(latest)
	return LatestInfo{
		Tag:         name,
		Version:     versionFromFilename(name),
		Path:        strings.Trim(latest.FullPath, "/"),
		PublishedAt: latest.PublishedAt,
		AssetsCount: count,
	}, true
}

func sourceForgeLatestNotFoundError(project, sourcePath string, files []File) error {
	base := fmt.Sprintf("could not determine SourceForge latest version for %s", project)
	if strings.TrimSpace(sourcePath) != "" {
		return fmt.Errorf("%s", base)
	}

	candidates := candidateDirectories(files)
	if len(candidates) == 0 {
		return fmt.Errorf("%s", base)
	}

	var b strings.Builder
	b.WriteString(base)
	b.WriteString("\ntry: eget q \"sf:")
	b.WriteString(project)
	b.WriteString("/")
	b.WriteString(strings.Trim(candidates[0].FullPath, "/"))
	b.WriteString("\"\ncandidate directories:")
	for _, candidate := range candidates {
		b.WriteString("\n  ")
		b.WriteString(sourceForgeReleaseTag(candidate))
	}
	return fmt.Errorf("%s", b.String())
}

func candidateDirectories(files []File) []File {
	candidates := make([]File, 0, len(files))
	for _, file := range files {
		if file.Type == TypeDirectory {
			candidates = append(candidates, file)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if !left.PublishedAt.Equal(right.PublishedAt) {
			return left.PublishedAt.After(right.PublishedAt)
		}
		return strings.ToLower(sourceForgeReleaseTag(left)) < strings.ToLower(sourceForgeReleaseTag(right))
	})
	return candidates
}

func versionFromFilename(name string) string {
	base := path.Base(strings.Trim(name, "/"))
	ext := path.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	if version := VersionFromText(base); version != "" {
		return version
	}
	return base
}

func sourceForgeReleaseTag(file File) string {
	if file.Name != "" {
		return file.Name
	}
	return path.Base(strings.Trim(file.FullPath, "/"))
}

func isPrereleaseVersion(file File) bool {
	name := strings.ToLower(sourceForgeReleaseTag(file))
	return strings.Contains(name, "alpha") ||
		strings.Contains(name, "beta") ||
		strings.Contains(name, "rc") ||
		strings.Contains(name, "pre") ||
		strings.Contains(name, "preview")
}

func latestPublishedAt(files []File) time.Time {
	var latest time.Time
	for _, file := range files {
		if file.Type != TypeFile || file.DownloadURL == "" || file.PublishedAt.IsZero() {
			continue
		}
		if latest.IsZero() || file.PublishedAt.After(latest) {
			latest = file.PublishedAt
		}
	}
	return latest
}

func stableDirectory(files []File) (File, bool) {
	for _, file := range files {
		if file.Type == TypeDirectory && strings.EqualFold(strings.Trim(file.Name, "/"), "stable") {
			return file, true
		}
	}
	return File{}, false
}

func (f Finder) list(sourcePath string) ([]File, error) {
	url := "https://sourceforge.net/projects/" + strings.Trim(f.Project, "/") + "/files/"
	if sourcePath != "" {
		url += escapeSourcePath(sourcePath) + "/"
	}

	verbosef("sourceforge finder request: %s", url)
	resp, err := f.Getter.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		files, rssErr := f.listRSS(sourcePath)
		if rssErr == nil {
			return files, nil
		}
		return nil, fmt.Errorf("sourceforge files page returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	verbosef("sourceforge finder response: %s", truncateBody(body))
	files, err := ParseFilesPage(body)
	if err == nil {
		return files, nil
	}
	files, rssErr := f.listRSS(sourcePath)
	if rssErr == nil {
		return files, nil
	}
	return nil, err
}

func (f Finder) listRSS(sourcePath string) ([]File, error) {
	u := "https://sourceforge.net/projects/" + strings.Trim(f.Project, "/") + "/rss?path=/"
	if sourcePath != "" {
		u = "https://sourceforge.net/projects/" + strings.Trim(f.Project, "/") + "/rss?path=" + url.QueryEscape("/"+strings.Trim(sourcePath, "/"))
	}
	verbosef("sourceforge rss request: %s", u)
	resp, err := f.Getter.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sourceforge rss returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	verbosef("sourceforge rss response: %s", truncateBody(body))
	return ParseRSSFilesPage(body)
}

func downloadableURLs(files []File) []string {
	urls := make([]string, 0, len(files))
	for _, file := range files {
		if file.Type == TypeFile && file.DownloadURL != "" {
			urls = append(urls, directDownloadURL(file))
		}
	}
	return urls
}

func prioritizeNewestFiles(files []File) {
	sort.SliceStable(files, func(i, j int) bool {
		left, right := fileVersion(files[i]), fileVersion(files[j])
		if left == "" {
			return false
		}
		if right == "" {
			return true
		}
		return compareVersion(left, right) > 0
	})
}

func directDownloadURL(file File) string {
	if strings.TrimSpace(file.FullPath) == "" {
		return file.DownloadURL
	}
	parsed, err := url.Parse(file.DownloadURL)
	if err != nil {
		return file.DownloadURL
	}

	if parsed.Host == "sourceforge.net" && strings.HasSuffix(strings.Trim(parsed.Path, "/"), "/download") {
		project := projectFromDownloadPath(parsed.Path)
		if project != "" {
			return sourceForgeDownloadURL(project, file.FullPath)
		}
	}

	if parsed.Host == "downloads.sourceforge.net" {
		project := projectFromDownloadsPath(parsed.Path)
		if project != "" {
			return sourceForgeDownloadURL(project, file.FullPath)
		}
	}

	return file.DownloadURL
}

func sourceForgeDownloadURL(project, fullPath string) string {
	return "https://downloads.sourceforge.net/project/" + url.PathEscape(project) + "/" + escapeSourcePath(fullPath)
}

func escapeSourcePath(sourcePath string) string {
	parts := strings.Split(strings.Trim(sourcePath, "/"), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func projectFromDownloadPath(rawPath string) string {
	parts := strings.Split(strings.Trim(rawPath, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "projects" {
			return path.Clean(parts[i+1])
		}
	}
	return ""
}

func projectFromDownloadsPath(rawPath string) string {
	parts := strings.Split(strings.Trim(rawPath, "/"), "/")
	if len(parts) >= 2 && parts[0] == "project" {
		return path.Clean(parts[1])
	}
	return ""
}
