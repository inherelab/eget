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
			for _, entry := range parseNodeFileDescriptor(file) {
				filename := fmt.Sprintf("node-v%s-%s-%s.%s", version, entry.os, entry.arch, entry.ext)
				rel, _ := url.Parse(path.Join(path.Dir(base.Path), "v"+version, filename))
				item.Files = append(item.Files, IndexFile{
					OS:       entry.os,
					Arch:     entry.arch,
					Ext:      entry.ext,
					URL:      base.ResolveReference(rel).String(),
					Filename: filename,
				})
			}
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

// nodeDescriptor is one archive a nodejs.org index.json file descriptor stands
// for. A descriptor names a platform, not a file: "linux-x64" is published as
// both node-vX-linux-x64.tar.gz and .tar.xz.
type nodeDescriptor struct {
	os   string
	arch string
	ext  string
}

// nodeExtToken maps the trailing token of a descriptor to the archive
// extension; "tar" is the .tar.gz form ("osx-x64-tar").
var nodeExtToken = map[string]string{
	"zip": "zip",
	"7z":  "7z",
	"tar": "tar.gz",
}

// nodeDescriptorOS maps descriptor OS tokens to the token used in file names
// ("osx-x64-tar" is published as node-vX-darwin-x64.tar.gz).
var nodeDescriptorOS = map[string]string{"osx": "darwin"}

// nodeNonArchiveDescriptors name entries that are not installable platform
// archives: the shared headers/source tarballs and the Windows/macOS installers
// ("win-x64-exe", "win-x64-msi", "osx-x64-pkg").
var nodeNonArchiveDescriptors = map[string]bool{"headers": true, "src": true}

// nodeTarballOS lists the platforms that publish both .tar.gz and .tar.xz; the
// rest (aix) publish only .tar.gz. Checked against
// nodejs.org/dist/{v22.9.0,v24.0.0,v26.12.0}/SHASUMS256.txt.
var nodeTarballOS = map[string]bool{"linux": true, "darwin": true}

// parseNodeFileDescriptor expands one nodejs.org index.json file descriptor into
// the index entries it stands for, e.g.
//
//	linux-x64       -> linux/x64/{tar.gz,tar.xz}
//	osx-x64-tar     -> darwin/x64/{tar.gz,tar.xz}
//	win-x64-zip     -> win/x64/zip
//	linux-x64-musl  -> linux/x64-musl/{tar.gz,tar.xz}
//
// Descriptors without an installable archive (headers, src, installers) yield
// nothing.
func parseNodeFileDescriptor(value string) []nodeDescriptor {
	desc := strings.ToLower(strings.TrimSpace(value))
	if desc == "" || nodeNonArchiveDescriptors[desc] {
		return nil
	}

	parts := strings.Split(desc, "-")
	if len(parts) < 2 {
		return nil
	}

	ext := ""
	if mapped, ok := nodeExtToken[parts[len(parts)-1]]; ok {
		ext = mapped
		parts = parts[:len(parts)-1]
	} else if last := parts[len(parts)-1]; last == "tar.gz" || last == "tar.xz" {
		ext = last
		parts = parts[:len(parts)-1]
	} else if last == "exe" || last == "msi" || last == "pkg" {
		// Installer, not an archive the SDK can extract.
		return nil
	}
	if len(parts) < 2 {
		return nil
	}

	osName := strings.Join(parts[:1], "")
	if mapped, ok := nodeDescriptorOS[osName]; ok {
		osName = mapped
	}
	arch := strings.Join(parts[1:], "-")
	if arch == "" {
		return nil
	}

	if ext != "" && ext != "tar.gz" {
		return []nodeDescriptor{{os: osName, arch: arch, ext: ext}}
	}
	// A bare descriptor, and the "-tar" form, name the platform's tarball: the
	// dist publishes .tar.gz for all of them and .tar.xz for the platforms that
	// build it.
	entries := []nodeDescriptor{{os: osName, arch: arch, ext: "tar.gz"}}
	if nodeTarballOS[osName] {
		entries = append(entries, nodeDescriptor{os: osName, arch: arch, ext: "tar.xz"})
	}
	return entries
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

// archiveExt returns the extension of an index file name without the leading
// dot: the multi-segment archive form when it matches, otherwise the last dot
// segment. The comparison is case-insensitive, so a source that ships
// "X.TAR.GZ" reports the same extension as its lower-case sibling.
func archiveExt(filename string) string {
	lower := strings.ToLower(filename)
	for _, ext := range filekind.ExtSetArchiveMulti {
		if strings.HasSuffix(lower, ext) {
			return ext[1:]
		}
	}
	return strings.TrimPrefix(path.Ext(lower), ".")
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
