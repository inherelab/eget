package sdk

import "time"

type VersionKind string

const (
	VersionLatest VersionKind = "latest"
	VersionExact  VersionKind = "exact"
	VersionPrefix VersionKind = "prefix"
)

type Target struct {
	Raw     string
	Name    string
	Version string
	Kind    VersionKind
}

type Index struct {
	Schema    int         `json:"schema"`
	SDK       string      `json:"sdk"`
	SourceURL string      `json:"source_url"`
	FetchedAt time.Time   `json:"fetched_at"`
	Items     []IndexItem `json:"items"`
}

type IndexItem struct {
	Version string      `json:"version"`
	Stable  bool        `json:"stable"`
	Files   []IndexFile `json:"files"`
}

type IndexFile struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Ext      string `json:"ext"`
	URL      string `json:"url"`
	Filename string `json:"filename"`
}

type SearchResult struct {
	SDK      string `json:"sdk"`
	Version  string `json:"version"`
	Stable   bool   `json:"stable"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Ext      string `json:"ext"`
	Filename string `json:"filename,omitempty"`
	URL      string `json:"url,omitempty"`
}

type SearchOptions struct {
	Keywords []string
	Number   int
	Sort     string
	// IncludePrerelease keeps non-stable versions in the results. By default
	// only stable versions are returned.
	IncludePrerelease bool
}

type IndexRefreshStage string

const (
	IndexRefreshFetchStart  IndexRefreshStage = "fetch_start"
	IndexRefreshFetchDone   IndexRefreshStage = "fetch_done"
	IndexRefreshParseStart  IndexRefreshStage = "parse_start"
	IndexRefreshParseDone   IndexRefreshStage = "parse_done"
	IndexRefreshParseFailed IndexRefreshStage = "parse_failed"
	IndexRefreshCacheHit    IndexRefreshStage = "cache_hit"
)

type IndexRefreshEvent struct {
	Stage    IndexRefreshStage
	SDK      string
	URL      string
	Format   string
	Parser   string
	Versions int
	Files    int
	Err      error
}

type InstalledStore struct {
	Schema    int                         `json:"schema"`
	Installed map[string]InstalledSDKNode `json:"installed"`
}

type InstalledSDKNode struct {
	Versions map[string]InstalledEntry `json:"versions"`
}

type InstalledEntry struct {
	Name            string    `json:"name"`
	Version         string    `json:"version"`
	Path            string    `json:"path"`
	URL             string    `json:"url"`
	Filename        string    `json:"filename"`
	OS              string    `json:"os"`
	Arch            string    `json:"arch"`
	Ext             string    `json:"ext"`
	InstalledAt     time.Time `json:"installed_at"`
	StripComponents int       `json:"strip_components"`
}
