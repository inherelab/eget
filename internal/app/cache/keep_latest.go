package cache

import (
	"path"
	"strconv"
	"strings"
)

var keepLatestOSTokens = []string{"windows", "win", "win32", "win64", "darwin", "macos", "osx", "linux", "freebsd", "openbsd", "netbsd", "android", "illumos", "solaris", "plan9"}
var keepLatestArchTokens = []string{"amd64", "x86_64", "x64", "386", "x86", "i386", "arm64", "aarch64", "arm32", "armv6", "armv7", "arm", "riscv64"}

type versionIdentifier struct {
	text    string
	number  uint64
	numeric bool
}

type assetVersion struct {
	core       []uint64
	prerelease []versionIdentifier
}

func (v assetVersion) stable() bool { return len(v.prerelease) == 0 }

type parsedCacheAsset struct {
	entry   Entry
	family  string
	rawVer  string
	version assetVersion
}

type keepLatestSelection struct {
	Matched      []Entry
	Kept         []Entry
	Unrecognized []Entry
}

type familyLatest struct {
	stable, prerelease       assetVersion
	hasStable, hasPrerelease bool
}

func selectKeepLatest(entries []Entry) keepLatestSelection {
	parsed := make([]parsedCacheAsset, 0, len(entries))
	latest := make(map[string]familyLatest)
	result := keepLatestSelection{}
	for _, entry := range entries {
		asset, ok := parseKeepLatestEntry(entry)
		if !ok {
			result.Unrecognized = append(result.Unrecognized, entry)
			continue
		}
		parsed = append(parsed, asset)
		group := latest[asset.family]
		if asset.version.stable() {
			if !group.hasStable || compareAssetVersion(asset.version, group.stable) > 0 {
				group.stable = asset.version
				group.hasStable = true
			}
		} else if !group.hasPrerelease || compareAssetVersion(asset.version, group.prerelease) > 0 {
			group.prerelease = asset.version
			group.hasPrerelease = true
		}
		latest[asset.family] = group
	}

	for _, asset := range parsed {
		group := latest[asset.family]
		keep := asset.version.stable() && compareAssetVersion(asset.version, group.stable) == 0
		keepPrerelease := group.hasPrerelease && (!group.hasStable || compareVersionCore(group.prerelease.core, group.stable.core) > 0)
		if !asset.version.stable() && keepPrerelease && compareAssetVersion(asset.version, group.prerelease) == 0 {
			keep = true
		}
		if keep {
			result.Kept = append(result.Kept, asset.entry)
		} else {
			result.Matched = append(result.Matched, asset.entry)
		}
	}
	return result
}

func parseAssetVersion(raw string) (assetVersion, bool) {
	if strings.HasPrefix(raw, "v") {
		raw = raw[1:]
	}
	if raw == "" || strings.Count(raw, "-") > 1 {
		return assetVersion{}, false
	}
	parts := strings.SplitN(raw, "-", 2)
	coreParts := strings.Split(parts[0], ".")
	if len(coreParts) < 2 {
		return assetVersion{}, false
	}
	v := assetVersion{core: make([]uint64, 0, len(coreParts))}
	for _, part := range coreParts {
		n, ok := parseVersionNumber(part)
		if !ok {
			return assetVersion{}, false
		}
		v.core = append(v.core, n)
	}
	if len(parts) == 1 {
		return v, true
	}
	ids := strings.Split(parts[1], ".")
	if len(ids) == 0 || strings.EqualFold(ids[0], "build") {
		return assetVersion{}, false
	}
	for _, id := range ids {
		if id == "" {
			return assetVersion{}, false
		}
		if n, ok := parseVersionNumber(id); ok {
			v.prerelease = append(v.prerelease, versionIdentifier{number: n, numeric: true})
			continue
		}
		if !isVersionText(id) {
			return assetVersion{}, false
		}
		v.prerelease = append(v.prerelease, versionIdentifier{text: id})
	}
	return v, true
}

func parseVersionNumber(s string) (uint64, bool) {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseUint(s, 10, 64)
	return n, err == nil
}

func isVersionText(s string) bool {
	if s[0] < 'A' || (s[0] > 'Z' && s[0] < 'a') || s[0] > 'z' {
		return false
	}
	for _, r := range s[1:] {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

func compareAssetVersion(a, b assetVersion) int {
	if cmp := compareVersionCore(a.core, b.core); cmp != 0 {
		return cmp
	}
	if a.stable() && b.stable() {
		return 0
	}
	if a.stable() {
		return 1
	}
	if b.stable() {
		return -1
	}
	return comparePrerelease(a.prerelease, b.prerelease)
}

func compareVersionCore(a, b []uint64) int {
	length := len(a)
	if len(b) > length {
		length = len(b)
	}
	for i := 0; i < length; i++ {
		var av, bv uint64
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func comparePrerelease(a, b []versionIdentifier) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i].numeric && b[i].numeric {
			if a[i].number < b[i].number {
				return -1
			}
			if a[i].number > b[i].number {
				return 1
			}
			continue
		}
		if a[i].numeric {
			return -1
		}
		if b[i].numeric {
			return 1
		}
		if a[i].text < b[i].text {
			return -1
		}
		if a[i].text > b[i].text {
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

func parseKeepLatestEntry(entry Entry) (parsedCacheAsset, bool) {
	if entry.Kind != KindPkg || entry.IsPartial || !strings.HasPrefix(entry.RelPath, "pkg-cache/") {
		return parsedCacheAsset{}, false
	}
	base := strings.TrimPrefix(entry.RelPath, "pkg-cache/")
	if base == "" || strings.Contains(base, "/") {
		return parsedCacheAsset{}, false
	}
	ext := cacheAssetExt(base)
	stem := strings.TrimSuffix(base, ext)
	idx := strings.LastIndexByte(stem, '-')
	if idx < 0 || !isLowerHex8(stem[idx+1:]) {
		return parsedCacheAsset{}, false
	}
	rawName, rawVer, ok := splitCacheAssetName(stem[:idx])
	if !ok {
		return parsedCacheAsset{}, false
	}
	version, ok := parseAssetVersion(rawVer)
	if !ok {
		return parsedCacheAsset{}, false
	}
	family, ok := normalizeAssetFamily(rawName, rawVer)
	if !ok {
		return parsedCacheAsset{}, false
	}
	return parsedCacheAsset{entry: entry, family: family, rawVer: rawVer, version: version}, true
}

func splitCacheAssetName(body string) (string, string, bool) {
	withoutPlatform := body
	if prefix, ok := trimPlatformSuffix(body, "-"); ok {
		withoutPlatform = prefix
	}
	for i := len(withoutPlatform) - 1; i >= 0; i-- {
		if withoutPlatform[i] != '-' {
			continue
		}
		candidate := withoutPlatform[i+1:]
		if _, ok := parseAssetVersion(candidate); ok && i > 0 {
			return withoutPlatform[:i], candidate, true
		}
	}
	return "", "", false
}

func normalizeAssetFamily(rawName, appendedVersion string) (string, bool) {
	name := rawName
	if prefix, ok := trimPlatformSuffix(name, "-"); ok {
		name = prefix
	}
	if prefix, ok := trimPlatformSuffix(name, "_"); ok {
		name = prefix
	}
	if prefix, ok := trimTrailingAssetVersion(name); ok {
		name = prefix
	}
	for _, version := range []string{appendedVersion, "v" + strings.TrimPrefix(appendedVersion, "v")} {
		if strings.HasSuffix(strings.ToLower(name), "-"+strings.ToLower(version)) {
			name = name[:len(name)-len(version)-1]
			break
		}
	}
	name = normalizeFamilySeparators(strings.ToLower(name))
	if name == "" {
		return "", false
	}
	for _, denied := range []string{"cli", "tool", "download", "release", "asset", "package", "app", "binary"} {
		if name == denied {
			return "", false
		}
	}
	return name, true
}

func trimTrailingAssetVersion(name string) (string, bool) {
	for i := len(name) - 1; i > 0; i-- {
		if name[i] != '-' {
			continue
		}
		if _, ok := parseAssetVersion(name[i+1:]); ok {
			return name[:i], true
		}
	}
	return name, false
}

func trimPlatformSuffix(name, sep string) (string, bool) {
	lower := strings.ToLower(name)
	for _, osToken := range keepLatestOSTokens {
		for _, archToken := range keepLatestArchTokens {
			suffix := sep + osToken + sep + archToken
			if strings.HasSuffix(lower, suffix) && len(name) > len(suffix) {
				return name[:len(name)-len(suffix)], true
			}
		}
	}
	return name, false
}

func normalizeFamilySeparators(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if r == '-' || r == '_' || r == '.' {
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
			continue
		}
		b.WriteRune(r)
		lastDash = false
	}
	return strings.Trim(b.String(), "-")
}

func cacheAssetExt(name string) string {
	lower := strings.ToLower(name)
	for _, ext := range []string{".tar.gz", ".tar.xz", ".tar.bz2", ".tar.zst", ".tar.br", ".tar.lz4"} {
		if strings.HasSuffix(lower, ext) {
			return name[len(name)-len(ext):]
		}
	}
	return path.Ext(name)
}

func isLowerHex8(s string) bool {
	if len(s) != 8 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}
