package client

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type CacheMeta struct {
	Name    string
	Version string
	OS      string
	Arch    string
}

func CacheFilePath(cacheDir, rawURL string) string {
	return CacheFilePathWithMeta(cacheDir, rawURL, CacheMeta{})
}

func CacheFilePathWithMeta(cacheDir, rawURL string, meta CacheMeta) string {
	if cacheDir == "" || rawURL == "" {
		return ""
	}
	u, _ := url.Parse(rawURL)
	fileName := cacheSourceFileName(rawURL, u)
	ext := archiveExt(fileName)
	if ext == "" {
		ext = ".bin"
	}
	name := strings.TrimSuffix(fileName, ext)
	if isGenericCacheName(name, ext) && meta.Name != "" {
		name = meta.Name
	}
	if name == "" {
		name = "download"
	}
	version := cacheVersion(rawURL, u, fileName, meta.Version)
	platform := cachePlatform(name, meta.OS, meta.Arch)
	if platform != "" {
		version = version + "-" + platform
	}
	return filepath.Join(cacheDir, "pkg-cache", fmt.Sprintf("%s-%s-%s%s", safeCachePart(name), safeCachePart(version), shortURLHash(rawURL), ext))
}

func APICacheFilePath(cacheDir, rawURL string) string {
	if cacheDir == "" || rawURL == "" {
		return ""
	}
	u, _ := url.Parse(rawURL)
	name := apiCacheName(rawURL, u)
	return filepath.Join(cacheDir, fmt.Sprintf("%s-%s.json", safeCachePart(name), shortURLHash(rawURL)))
}

func cacheSourceFileName(rawURL string, u *url.URL) string {
	if u != nil && u.Path != "" {
		if base := path.Base(strings.TrimRight(u.Path, "/")); base != "." && base != "/" {
			return base
		}
	}
	base := path.Base(strings.TrimRight(rawURL, "/"))
	if idx := strings.IndexAny(base, "?#"); idx >= 0 {
		base = base[:idx]
	}
	if base == "" || base == "." || base == "/" {
		return "download"
	}
	return base
}

func archiveExt(name string) string {
	lower := strings.ToLower(name)
	for _, ext := range []string{".tar.gz", ".tar.xz", ".tar.bz2", ".tar.zst", ".tar.br", ".tar.lz4"} {
		if strings.HasSuffix(lower, ext) {
			return name[len(name)-len(ext):]
		}
	}
	return path.Ext(name)
}

func cacheVersion(rawURL string, u *url.URL, fileName, fallback string) string {
	if u != nil {
		parts := strings.Split(strings.Trim(u.EscapedPath(), "/"), "/")
		for i := 0; i+2 < len(parts); i++ {
			if parts[i] == "releases" && parts[i+1] == "download" {
				if tag, err := url.PathUnescape(parts[i+2]); err == nil && tag != "" {
					return trimVersionPrefix(tag)
				}
				return trimVersionPrefix(parts[i+2])
			}
		}
	}
	if fallback != "" {
		return trimVersionPrefix(fallback)
	}
	if version := versionFromText(fileName); version != "" {
		return trimVersionPrefix(version)
	}
	if version := versionFromText(rawURL); version != "" {
		return trimVersionPrefix(version)
	}
	return "unknown"
}

func isGenericCacheName(name, ext string) bool {
	if ext != ".bin" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "download", "latest", "release", "asset", "file":
		return true
	default:
		return false
	}
}

func cachePlatform(name, goos, goarch string) string {
	goos = strings.ToLower(strings.TrimSpace(goos))
	goarch = strings.ToLower(strings.TrimSpace(goarch))
	if goos == "" || goarch == "" || nameHasPlatform(name) {
		return ""
	}
	return safeCachePart(goos) + "-" + safeCachePart(goarch)
}

func nameHasPlatform(name string) bool {
	tokens := cacheNameTokens(name)
	return hasAnyCacheToken(tokens, cacheOSTokens...) || hasAnyCacheToken(tokens, cacheArchTokens...)
}

func cacheNameTokens(name string) []string {
	lower := strings.ToLower(name)
	return strings.FieldsFunc(lower, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == '/' || r == '\\'
	})
}

func hasAnyCacheToken(tokens []string, aliases ...string) bool {
	for _, token := range tokens {
		for _, alias := range aliases {
			if token == alias {
				return true
			}
		}
	}
	return false
}

var cacheOSTokens = []string{"windows", "win", "win32", "win64", "darwin", "macos", "osx", "linux", "freebsd", "openbsd", "netbsd", "android", "illumos", "solaris", "plan9"}
var cacheArchTokens = []string{"amd64", "x86_64", "x64", "386", "x86", "i386", "arm64", "aarch64", "arm32", "armv6", "armv7", "arm", "riscv64"}

var cacheVersionPattern = regexp.MustCompile(`(?i)(?:^|[^0-9])v?(\d+(?:\.\d+)+(?:[-+][0-9A-Za-z.-]+)?)`)

func versionFromText(text string) string {
	match := cacheVersionPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func trimVersionPrefix(version string) string {
	return strings.TrimPrefix(strings.TrimPrefix(version, "v"), "V")
}

func apiCacheName(rawURL string, u *url.URL) string {
	if u != nil && u.Host != "" {
		parts := []string{u.Host}
		for _, part := range strings.Split(strings.Trim(u.Path, "/"), "/") {
			if part != "" {
				parts = append(parts, part)
			}
		}
		return strings.Join(parts, "-")
	}
	name := strings.TrimSpace(rawURL)
	if name == "" {
		return "api"
	}
	return name
}

func shortURLHash(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(sum[:])[:8]
}

func safeCachePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '.' || r == '_' || r == '-'
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		return "unknown"
	}
	return out
}
