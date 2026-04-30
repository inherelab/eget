package forge

import (
	"encoding/json"
	"net/url"
	"strings"
)

type gitLabRelease struct {
	Tag    string `json:"tag_name"`
	Assets struct {
		Links []struct {
			Name           string `json:"name"`
			URL            string `json:"url"`
			DirectAssetURL string `json:"direct_asset_url"`
		} `json:"links"`
	} `json:"assets"`
}

func (f Finder) gitLabRelease() (releaseInfo, error) {
	apiURL := gitLabReleaseURL(f.Target, f.Tag)
	body, err := f.getJSON(apiURL)
	if err != nil {
		return releaseInfo{}, err
	}

	var release gitLabRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return releaseInfo{}, err
	}

	assets := make([]string, 0, len(release.Assets.Links))
	for _, link := range release.Assets.Links {
		assetURL := strings.TrimSpace(link.DirectAssetURL)
		if assetURL == "" {
			assetURL = strings.TrimSpace(link.URL)
		}
		if assetURL != "" {
			assets = append(assets, assetURL)
		}
	}
	verbosef("forge gitlab assets: %d", len(assets))
	return releaseInfo{Tag: release.Tag, Assets: assets}, nil
}

func gitLabReleaseURL(target Target, tag string) string {
	projectPath := target.Namespace + "/" + target.Project
	base := "https://" + target.Host + "/api/v4/projects/" + url.PathEscape(projectPath)
	if tag == "" {
		return base + "/releases/permalink/latest"
	}
	return base + "/releases/" + url.PathEscape(tag)
}
