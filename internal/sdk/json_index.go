package sdk

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/inherelab/eget/internal/filekind"
)

type JSONParseOptions struct {
	SDK       string
	SourceURL string
	Now       func() time.Time
}

func ParseJSONIndex(body io.Reader, parser string, opts JSONParseOptions) (Index, error) {
	switch parser {
	case "go-json":
		return parseGoJSONIndex(body, opts)
	case "node-json":
		return parseNodeJSONIndex(body, opts)
	case "zulu-json":
		return parseZuluJSONIndex(body, opts)
	default:
		return Index{}, fmt.Errorf("unsupported sdk json index parser %q", parser)
	}
}

type goJSONRelease struct {
	Version string       `json:"version"`
	Stable  bool         `json:"stable"`
	Files   []goJSONFile `json:"files"`
}

type goJSONFile struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Kind     string `json:"kind"`
}

func parseGoJSONIndex(body io.Reader, opts JSONParseOptions) (Index, error) {
	var releases []goJSONRelease
	if err := json.NewDecoder(body).Decode(&releases); err != nil {
		return Index{}, err
	}

	items := make([]IndexItem, 0, len(releases))
	for _, release := range releases {
		version := strings.TrimPrefix(release.Version, "go")
		item := IndexItem{Version: version, Stable: release.Stable}
		for _, file := range release.Files {
			if file.Kind != "" && file.Kind != "archive" {
				continue
			}
			item.Files = append(item.Files, IndexFile{
				OS:       file.OS,
				Arch:     file.Arch,
				Ext:      archiveExt(file.Filename),
				URL:      "https://go.dev/dl/" + file.Filename,
				Filename: file.Filename,
			})
		}
		if len(item.Files) > 0 {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return Index{}, fmt.Errorf("no sdk files found in json index")
	}
	return newIndex(opts.SDK, opts.SourceURL, opts.Now, items), nil
}

type nodeJSONRelease struct {
	Version string   `json:"version"`
	Files   []string `json:"files"`
}

func parseNodeJSONIndex(body io.Reader, opts JSONParseOptions) (Index, error) {
	var releases []nodeJSONRelease
	if err := json.NewDecoder(body).Decode(&releases); err != nil {
		return Index{}, err
	}
	base, err := url.Parse(opts.SourceURL)
	if err != nil {
		return Index{}, err
	}

	items := make([]IndexItem, 0, len(releases))
	for _, release := range releases {
		version := strings.TrimPrefix(release.Version, "v")
		item := IndexItem{Version: version, Stable: isStableVersion(version)}
		for _, file := range release.Files {
			osName, arch, ext, ok := parseNodeFileDescriptor(file)
			if !ok {
				continue
			}
			filename := fmt.Sprintf("node-v%s-%s-%s.%s", version, osName, arch, ext)
			rel, _ := url.Parse(path.Join(path.Dir(base.Path), "v"+version, filename))
			item.Files = append(item.Files, IndexFile{
				OS:       osName,
				Arch:     arch,
				Ext:      ext,
				URL:      base.ResolveReference(rel).String(),
				Filename: filename,
			})
		}
		if len(item.Files) > 0 {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return Index{}, fmt.Errorf("no sdk files found in json index")
	}
	return newIndex(opts.SDK, opts.SourceURL, opts.Now, items), nil
}

func parseNodeFileDescriptor(value string) (string, string, string, bool) {
	switch {
	case strings.HasSuffix(value, "-tar.xz"):
		parts := strings.Split(strings.TrimSuffix(value, "-tar.xz"), "-")
		if len(parts) != 2 {
			return "", "", "", false
		}
		return parts[0], parts[1], "tar.xz", true
	case strings.HasSuffix(value, "-tar.gz"):
		parts := strings.Split(strings.TrimSuffix(value, "-tar.gz"), "-")
		if len(parts) != 2 {
			return "", "", "", false
		}
		return parts[0], parts[1], "tar.gz", true
	case strings.HasSuffix(value, "-zip"):
		parts := strings.Split(strings.TrimSuffix(value, "-zip"), "-")
		if len(parts) != 2 {
			return "", "", "", false
		}
		return parts[0], parts[1], "zip", true
	default:
		return "", "", "", false
	}
}

type zuluJSONPackage struct {
	DownloadURL string `json:"download_url"`
	Name        string `json:"name"`
	JavaVersion []int  `json:"java_version"`
}

func parseZuluJSONIndex(body io.Reader, opts JSONParseOptions) (Index, error) {
	var packages []zuluJSONPackage
	if err := json.NewDecoder(body).Decode(&packages); err != nil {
		return Index{}, err
	}

	byVersion := make(map[string]*IndexItem)
	order := make([]string, 0)
	for _, pkg := range packages {
		version := zuluJavaVersion(pkg.JavaVersion)
		osName, arch, ext, ok := parseZuluFilename(pkg.Name)
		if version == "" || pkg.DownloadURL == "" || !ok || !isZuluStandardJDKPackage(pkg.Name) {
			continue
		}
		item := byVersion[version]
		if item == nil {
			item = &IndexItem{Version: version, Stable: true}
			byVersion[version] = item
			order = append(order, version)
		}
		item.Files = append(item.Files, IndexFile{
			OS:       osName,
			Arch:     arch,
			Ext:      ext,
			URL:      pkg.DownloadURL,
			Filename: pkg.Name,
		})
	}

	items := make([]IndexItem, 0, len(order))
	for _, version := range order {
		item := byVersion[version]
		if len(item.Files) > 0 {
			items = append(items, *item)
		}
	}
	if len(items) == 0 {
		return Index{}, fmt.Errorf("no sdk files found in json index")
	}
	return newIndex(opts.SDK, opts.SourceURL, opts.Now, items), nil
}

func isZuluStandardJDKPackage(filename string) bool {
	return strings.Contains(filename, "-ca-jdk")
}

func zuluJavaVersion(values []int) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprint(value))
	}
	return strings.Join(parts, ".")
}

func parseZuluFilename(filename string) (string, string, string, bool) {
	ext := archiveExt(filename)
	if ext != "tar.gz" && ext != "zip" {
		return "", "", "", false
	}
	name := strings.TrimSuffix(filename, "."+ext)
	platform := name[strings.LastIndex(name, "-")+1:]
	osName, arch, found := strings.Cut(platform, "_")
	if !found || osName == "" || arch == "" {
		return "", "", "", false
	}
	return osName, arch, ext, true
}

func archiveExt(filename string) string {
	ext, _ := filekind.MultiExtExact(filename, filekind.ExtSetArchiveLegacy)
	return strings.TrimPrefix(ext, ".")
}

func newIndex(sdkName, sourceURL string, now func() time.Time, items []IndexItem) Index {
	if now == nil {
		now = time.Now
	}
	return Index{
		Schema:    1,
		SDK:       sdkName,
		SourceURL: sourceURL,
		FetchedAt: now(),
		Items:     items,
	}
}
