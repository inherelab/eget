package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var verboseWriter io.Writer = os.Stderr
var verboseEnabled bool

func SetVerbose(enabled bool, writer io.Writer) {
	verboseEnabled = enabled
	if writer == nil {
		verboseWriter = io.Discard
		return
	}
	verboseWriter = writer
}

type HTTPGetter interface {
	Get(url string) (*http.Response, error)
}

type Finder interface {
	Find() ([]string, error)
}

type Release struct {
	Assets []struct {
		DownloadURL string `json:"browser_download_url"`
	} `json:"assets"`

	Prerelease bool      `json:"prerelease"`
	Tag        string    `json:"tag_name"`
	CreatedAt  time.Time `json:"created_at"`
}

type Error struct {
	Code   int
	Status string
	Body   []byte
	URL    string
}

type errResponse struct {
	Message string `json:"message"`
	Doc     string `json:"documentation_url"`
}

func (e *Error) Error() string {
	var msg errResponse
	_ = json.Unmarshal(e.Body, &msg)

	if e.Code == http.StatusForbidden {
		return fmt.Sprintf("%s: %s: %s", e.Status, msg.Message, msg.Doc)
	}
	return fmt.Sprintf("%s (URL: %s)", e.Status, e.URL)
}

type AssetFinder struct {
	Repo       string
	Tag        string
	Prerelease bool
	MinTime    time.Time
	Getter     HTTPGetter
}

var ErrNoUpgrade = errors.New("requested release is not more recent than current version")

func NewAssetFinder(repo, tag string, prerelease bool, minTime time.Time) *AssetFinder {
	return &AssetFinder{
		Repo:       repo,
		Tag:        tag,
		Prerelease: prerelease,
		MinTime:    minTime,
	}
}

func (f *AssetFinder) Find() ([]string, error) {
	if f.Getter == nil {
		return nil, fmt.Errorf("github getter is required")
	}
	if f.Prerelease && f.Tag == "latest" {
		tag, err := f.getLatestTag()
		if err != nil {
			return nil, err
		}
		f.Tag = fmt.Sprintf("tags/%s", tag)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/%s", f.Repo, f.Tag)
	verbosef("github finder request: %s", url)
	resp, err := f.Getter.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(f.Tag, "tags/") && resp.StatusCode == http.StatusNotFound {
			return f.FindMatch()
		}
		return nil, &Error{Status: resp.Status, Code: resp.StatusCode, Body: body, URL: url}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	verbosef("github finder response: %s", truncateBody(body))

	var release Release
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, err
	}

	if release.CreatedAt.Before(f.MinTime) {
		return nil, ErrNoUpgrade
	}

	assets := make([]string, 0, len(release.Assets))
	for _, a := range release.Assets {
		assets = append(assets, a.DownloadURL)
	}
	verbosef("github finder assets: %d", len(assets))

	return assets, nil
}

func (f *AssetFinder) FindMatch() ([]string, error) {
	if f.Getter == nil {
		return nil, fmt.Errorf("github getter is required")
	}
	tag := strings.TrimPrefix(f.Tag, "tags/")

	for page := 1; ; page++ {
		url := fmt.Sprintf("https://api.github.com/repos/%s/releases?page=%d", f.Repo, page)
		verbosef("github finder fallback request: %s", url)
		resp, err := f.Getter.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}
			return nil, &Error{Status: resp.Status, Code: resp.StatusCode, Body: body, URL: url}
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		verbosef("github finder fallback response: %s", truncateBody(body))

		var releases []Release
		if err := json.Unmarshal(body, &releases); err != nil {
			return nil, err
		}

		for _, r := range releases {
			if !f.Prerelease && r.Prerelease {
				continue
			}
			if strings.Contains(r.Tag, tag) && !r.CreatedAt.Before(f.MinTime) {
				assets := make([]string, 0, len(r.Assets))
				for _, a := range r.Assets {
					assets = append(assets, a.DownloadURL)
				}
				return assets, nil
			}
		}

		if len(releases) < 30 {
			break
		}
	}

	return nil, fmt.Errorf("no matching tag for '%s'", tag)
}

func (f *AssetFinder) getLatestTag() (string, error) {
	if f.Getter == nil {
		return "", fmt.Errorf("github getter is required")
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases", f.Repo)
	verbosef("github prerelease request: %s", url)
	resp, err := f.Getter.Get(url)
	if err != nil {
		return "", fmt.Errorf("pre-release finder: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("pre-release finder: %w", err)
	}
	verbosef("github prerelease response: %s", truncateBody(body))

	var releases []Release
	if err := json.Unmarshal(body, &releases); err != nil {
		return "", fmt.Errorf("pre-release finder: %w", err)
	}

	if len(releases) == 0 {
		return "", fmt.Errorf("no releases found")
	}

	return releases[0].Tag, nil
}

type SourceFinder struct {
	Tool string
	Repo string
	Tag  string
}

func NewSourceFinder(repo, tag, tool string) *SourceFinder {
	return &SourceFinder{Repo: repo, Tag: tag, Tool: tool}
}

func (f *SourceFinder) Find() ([]string, error) {
	return []string{fmt.Sprintf("https://github.com/%s/tarball/%s/%s.tar.gz", f.Repo, f.Tag, f.Tool)}, nil
}

func verbosef(format string, args ...any) {
	if !verboseEnabled || verboseWriter == nil {
		return
	}
	fmt.Fprintf(verboseWriter, "[verbose] "+format+"\n", args...)
}

func truncateBody(body []byte) string {
	const limit = 240
	text := strings.TrimSpace(string(body))
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "...(truncated)"
}

func VerboseEnabledForTest() bool {
	return verboseEnabled
}
