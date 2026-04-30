package forge

import (
	"encoding/json"
	"net/url"
	"strings"
)

type giteaRelease struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func (f Finder) giteaRelease() (releaseInfo, error) {
	apiURL := giteaReleaseURL(f.Target, f.Tag)
	body, err := f.getJSON(apiURL)
	if err != nil {
		return releaseInfo{}, err
	}

	var release giteaRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return releaseInfo{}, err
	}

	assets := make([]string, 0, len(release.Assets))
	for _, asset := range release.Assets {
		assetURL := strings.TrimSpace(asset.BrowserDownloadURL)
		if assetURL != "" {
			assets = append(assets, assetURL)
		}
	}
	verbosef("forge %s assets: %d", f.Target.Provider, len(assets))
	return releaseInfo{Tag: release.Tag, Assets: assets}, nil
}

func giteaReleaseURL(target Target, tag string) string {
	base := "https://" + target.Host + "/api/v1/repos/" + target.Namespace + "/" + target.Project + "/releases/"
	if tag == "" {
		return base + "latest"
	}
	return base + "tags/" + url.PathEscape(tag)
}
