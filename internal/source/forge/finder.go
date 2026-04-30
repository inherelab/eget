package forge

import (
	"fmt"
	"io"
	"net/http"
)

type HTTPGetter interface {
	Get(url string) (*http.Response, error)
}

type Finder struct {
	Target Target
	Tag    string
	Getter HTTPGetter
}

type LatestInfo struct {
	Tag string
}

type releaseInfo struct {
	Tag    string
	Assets []string
}

func (f Finder) Find() ([]string, error) {
	if f.Getter == nil {
		return nil, fmt.Errorf("forge HTTP getter is required")
	}
	release, err := f.release()
	if err != nil {
		return nil, err
	}
	if len(release.Assets) == 0 {
		return nil, fmt.Errorf("%s release assets not found for %s/%s/%s", f.Target.Provider, f.Target.Host, f.Target.Namespace, f.Target.Project)
	}
	return release.Assets, nil
}

func LatestVersion(target Target, getter HTTPGetter) (LatestInfo, error) {
	release, err := Finder{Target: target, Getter: getter}.release()
	if err != nil {
		return LatestInfo{}, err
	}
	if release.Tag == "" {
		return LatestInfo{}, fmt.Errorf("%s latest release tag not found for %s/%s/%s", target.Provider, target.Host, target.Namespace, target.Project)
	}
	return LatestInfo{Tag: release.Tag}, nil
}

func (f Finder) release() (releaseInfo, error) {
	switch f.Target.Provider {
	case ProviderGitLab:
		return f.gitLabRelease()
	case ProviderGitea, ProviderForgejo:
		return releaseInfo{}, fmt.Errorf("%s release finder is not implemented", f.Target.Provider)
	default:
		return releaseInfo{}, fmt.Errorf("unsupported forge provider %q", f.Target.Provider)
	}
}

func (f Finder) getJSON(rawURL string) ([]byte, error) {
	verbosef("forge %s request: %s", f.Target.Provider, rawURL)
	resp, err := f.Getter.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	verbosef("forge %s response: %s", f.Target.Provider, truncateBody(body))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s release request failed: %d %s (URL: %s)", f.Target.Provider, resp.StatusCode, http.StatusText(resp.StatusCode), rawURL)
	}
	return body, nil
}
