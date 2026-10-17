package filekind

import (
	"path"
	"path/filepath"
	"strings"
)

// KindExact reports the archive kind of name using the case-sensitive suffix
// list of install.NewExtractor; it returns KindUnknown when nothing matches.
func KindExact(name string) Kind {
	for _, item := range kindSuffixes {
		if strings.HasSuffix(name, item.ext) {
			return item.kind
		}
	}
	return KindUnknown
}

// MultiExtFold returns the extension of name for the multi-segment set, matching
// case-insensitively and returning the slice of the original input (so its case
// is preserved) — the exact behaviour of client.archiveExt and
// app/cache.cacheAssetExt. Without a match it falls back to path.Ext.
func MultiExtFold(name string, set []string) string {
	lower := strings.ToLower(name)
	for _, ext := range set {
		if strings.HasSuffix(lower, ext) {
			return name[len(name)-len(ext):]
		}
	}
	return path.Ext(name)
}

// MultiExtExact returns the extension of name for the multi-segment set,
// matching case-sensitively. On a match it returns the set literal, e.g.
// ".tar.gz"; callers trim the leading dot to get the sdk-style value. Without a
// match it returns path.Ext(name) and false.
func MultiExtExact(name string, set []string) (string, bool) {
	for _, ext := range set {
		if strings.HasSuffix(name, ext) {
			return ext, true
		}
	}
	return path.Ext(name), false
}

// IsArchiveFold reports whether the folded name ends with a suffix of set.
func IsArchiveFold(name string, set []string) bool {
	lower := strings.ToLower(name)
	for _, ext := range set {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// IsDownloadFold reports whether the folded extension of name (a file system
// path, as consumed by install.isInvalidCachedDownload) is a downloadable
// archive entry.
func IsDownloadFold(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, candidate := range ExtSetDownload {
		if ext == candidate {
			return true
		}
	}
	return false
}
