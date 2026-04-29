package sourceforge

import (
	"fmt"
	"io"
	"net/http"
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

func (f Finder) list(sourcePath string) ([]File, error) {
	url := "https://sourceforge.net/projects/" + strings.Trim(f.Project, "/") + "/files/"
	if sourcePath != "" {
		url += strings.Trim(sourcePath, "/") + "/"
	}

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
	return ParseFilesPage(body)
}

func downloadableURLs(files []File) []string {
	urls := make([]string, 0, len(files))
	for _, file := range files {
		if file.Type == TypeFile && file.DownloadURL != "" {
			urls = append(urls, file.URL)
		}
	}
	return urls
}
