package sourceforge

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
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
	Version string
	Path    string
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

	urls = downloadableURLs(files)
	if len(urls) == 0 {
		return nil, fmt.Errorf("sourceforge downloadable files not found")
	}
	return urls, nil
}

func LatestVersion(project, sourcePath string, getter HTTPGetter) (LatestInfo, error) {
	finder := Finder{Project: project, Path: sourcePath, Getter: getter}
	files, err := finder.list(strings.Trim(sourcePath, "/"))
	if err != nil {
		return LatestInfo{}, err
	}

	latest, ok := LatestVersionFile(files)
	if !ok {
		if sourcePath == "" {
			stable, stableOK := stableDirectory(files)
			if stableOK {
				files, err = finder.list(stable.FullPath)
				if err != nil {
					return LatestInfo{}, err
				}
				latest, ok = LatestVersionFile(files)
			}
		}
	}
	if !ok {
		return LatestInfo{}, fmt.Errorf("could not determine SourceForge latest version for %s", project)
	}
	version := VersionFromText(latest.Name)
	if version == "" {
		version = VersionFromText(latest.FullPath)
	}
	if version == "" {
		return LatestInfo{}, fmt.Errorf("could not determine SourceForge latest version for %s", project)
	}
	return LatestInfo{Version: version, Path: latest.FullPath}, nil
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
		url += strings.Trim(sourcePath, "/") + "/"
	}

	verbosef("sourceforge finder request: %s", url)
	resp, err := f.Getter.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sourceforge files page returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	verbosef("sourceforge finder response: %s", truncateBody(body))
	return ParseFilesPage(body)
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

func directDownloadURL(file File) string {
	if strings.TrimSpace(file.FullPath) == "" {
		return file.DownloadURL
	}
	parsed, err := url.Parse(file.DownloadURL)
	if err != nil || parsed.Host != "sourceforge.net" || !strings.HasSuffix(strings.Trim(parsed.Path, "/"), "/download") {
		return file.DownloadURL
	}

	project := projectFromDownloadPath(parsed.Path)
	if project == "" {
		return file.DownloadURL
	}
	return "https://downloads.sourceforge.net/project/" + project + "/" + strings.Trim(file.FullPath, "/")
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
