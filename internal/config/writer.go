package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Save(path string, file *File) error {
	if path == "" {
		return fmt.Errorf("config path is required")
	}
	if file == nil {
		file = NewFile()
	}

	var buf bytes.Buffer
	writeSectionHeader(&buf, "global")
	writeSectionBody(&buf, file.Global)
	if hasAPICacheConfig(file.ApiCache) {
		buf.WriteString("\n")
		writeSectionHeader(&buf, "api_cache")
		writeAPICacheBody(&buf, file.ApiCache)
	}
	if hasGhproxyConfig(file.Ghproxy) {
		buf.WriteString("\n")
		writeSectionHeader(&buf, "ghproxy")
		writeGhproxyBody(&buf, file.Ghproxy)
	}

	repoNames := sortedKeys(file.Repos)
	for _, name := range repoNames {
		buf.WriteString("\n")
		writeSectionHeader(&buf, quoteKey(name))
		writeSectionBody(&buf, file.Repos[name])
	}

	buf.WriteString("\n[packages]\n")
	pkgNames := sortedKeys(file.Packages)
	for _, name := range pkgNames {
		buf.WriteString("\n")
		writeSectionHeader(&buf, "packages."+quoteKey(name))
		writeSectionBody(&buf, file.Packages[name])
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func writeSectionHeader(buf *bytes.Buffer, name string) {
	buf.WriteString("[")
	buf.WriteString(name)
	buf.WriteString("]\n")
}

func writeSectionBody(buf *bytes.Buffer, section Section) {
	writeBool(buf, "all", section.All)
	writeStrings(buf, "asset_filters", section.AssetFilters)
	writeString(buf, "cache_dir", section.CacheDir)
	writeString(buf, "proxy_url", section.ProxyURL)
	writeBool(buf, "download_only", section.DownloadOnly)
	writeString(buf, "file", section.File)
	writeString(buf, "github_token", section.GithubToken)
	writeString(buf, "name", section.Name)
	writeBool(buf, "quiet", section.Quiet)
	writeString(buf, "repo", section.Repo)
	writeBool(buf, "show_hash", section.ShowHash)
	writeBool(buf, "download_source", section.Source)
	writeString(buf, "system", section.System)
	writeString(buf, "tag", section.Tag)
	writeString(buf, "target", section.Target)
	writeBool(buf, "upgrade_only", section.UpgradeOnly)
	writeString(buf, "verify_sha256", section.Verify)
	writeBool(buf, "disable_ssl", section.DisableSSL)
}

func writeAPICacheBody(buf *bytes.Buffer, section APICacheSection) {
	writeBool(buf, "enable", section.Enable)
	writeInt(buf, "cache_time", section.CacheTime)
}

func writeGhproxyBody(buf *bytes.Buffer, section GhproxySection) {
	writeBool(buf, "enable", section.Enable)
	writeString(buf, "host_url", section.HostURL)
	writeBool(buf, "support_api", section.SupportAPI)
	writeStrings(buf, "fallbacks", section.Fallbacks)
}

func writeBool(buf *bytes.Buffer, key string, value *bool) {
	if value == nil {
		return
	}
	fmt.Fprintf(buf, "%s = %t\n", key, *value)
}

func writeString(buf *bytes.Buffer, key string, value *string) {
	if value == nil {
		return
	}
	fmt.Fprintf(buf, "%s = %q\n", key, *value)
}

func writeInt(buf *bytes.Buffer, key string, value *int) {
	if value == nil {
		return
	}
	fmt.Fprintf(buf, "%s = %d\n", key, *value)
}

func writeStrings(buf *bytes.Buffer, key string, values []string) {
	if len(values) == 0 {
		return
	}
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = fmt.Sprintf("%q", value)
	}
	fmt.Fprintf(buf, "%s = [%s]\n", key, strings.Join(quoted, ", "))
}

func sortedKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func quoteKey(key string) string {
	return fmt.Sprintf("%q", key)
}

func hasAPICacheConfig(section APICacheSection) bool {
	return section.Enable != nil || section.CacheTime != nil
}

func hasGhproxyConfig(section GhproxySection) bool {
	return section.Enable != nil || section.HostURL != nil || section.SupportAPI != nil || len(section.Fallbacks) > 0
}
