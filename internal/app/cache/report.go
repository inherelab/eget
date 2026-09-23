package cache

import (
	"sort"
	"strings"
	"time"

	"github.com/inherelab/eget/internal/cachemirror"
	"github.com/inherelab/eget/internal/util"
)

type ListOptions struct {
	Root string
}

type ListFile struct {
	Kind    Kind      `json:"kind"`
	Path    string    `json:"path"`
	PathKey string    `json:"path_key,omitempty"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

type ListResult struct {
	CacheDir   string     `json:"cache_dir"`
	Root       string     `json:"root"`
	TotalFiles int        `json:"total_files"`
	TotalSize  int64      `json:"total_size"`
	Files      []ListFile `json:"files"`
}

type KindSummary struct {
	Files int   `json:"files"`
	Size  int64 `json:"size"`
}

type CacheMirrorStatus struct {
	Enable   bool   `json:"enable"`
	URL      string `json:"url,omitempty"`
	Timeout  int    `json:"timeout,omitempty"`
	Fallback bool   `json:"fallback"`
}

type StatusResult struct {
	CacheDir     string                 `json:"cache_dir"`
	GeneratedAt  time.Time              `json:"generated_at"`
	TotalFiles   int                    `json:"total_files"`
	TotalSize    int64                  `json:"total_size"`
	Kinds        map[string]KindSummary `json:"kinds"`
	ServeCommand string                 `json:"serve_command"`
	CacheMirror  CacheMirrorStatus      `json:"cache_mirror"`
}

func (s Service) List(cacheDir string, opts ListOptions) (ListResult, error) {
	resolved, err := s.resolveCacheDirInput(cacheDir)
	if err != nil {
		return ListResult{}, err
	}
	root := normalizeReportRoot(opts.Root)
	entries, err := s.Scan(resolved, CacheScanOptions{Kinds: reportKindsForRoot(root)})
	if err != nil {
		return ListResult{}, err
	}

	files := make([]ListFile, 0, len(entries))
	var totalSize int64
	for _, entry := range entries {
		if root != "" && root != "all" && root != string(entry.Kind) {
			continue
		}
		file := ListFile{
			Kind:    entry.Kind,
			Path:    entry.RelPath,
			Size:    entry.Size,
			ModTime: entry.ModTime,
		}
		if entry.Kind != KindPartial {
			file.PathKey = cachemirror.KeyForRelPath(entry.RelPath)
		}
		files = append(files, file)
		totalSize += entry.Size
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return ListResult{
		CacheDir:   resolved,
		Root:       root,
		TotalFiles: len(files),
		TotalSize:  totalSize,
		Files:      files,
	}, nil
}

func (s Service) Status(cacheDir string) (StatusResult, error) {
	resolved, err := s.resolveCacheDirInput(cacheDir)
	if err != nil {
		return StatusResult{}, err
	}
	entries, err := s.Scan(resolved, CacheScanOptions{Kinds: []Kind{
		KindPkg,
		KindAPI,
		KindSDK,
		KindSDKIndex,
		KindPartial,
	}})
	if err != nil {
		return StatusResult{}, err
	}

	kinds := make(map[string]KindSummary, 5)
	var totalFiles int
	var totalSize int64
	for _, entry := range entries {
		key := string(entry.Kind)
		summary := kinds[key]
		summary.Files++
		summary.Size += entry.Size
		kinds[key] = summary
		totalFiles++
		totalSize += entry.Size
	}

	return StatusResult{
		CacheDir:     resolved,
		GeneratedAt:  s.now(),
		TotalFiles:   totalFiles,
		TotalSize:    totalSize,
		Kinds:        kinds,
		ServeCommand: "eget web --host 0.0.0.0 --port 8787",
		CacheMirror:  s.cacheMirrorStatus(),
	}, nil
}

func (s Service) resolveCacheDirInput(cacheDir string) (string, error) {
	if strings.TrimSpace(cacheDir) != "" {
		return cacheDir, nil
	}
	return s.ResolveCacheDir()
}

func normalizeReportRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return "all"
	}
	return root
}

func reportKindsForRoot(root string) []Kind {
	switch root {
	case string(KindPkg):
		return []Kind{KindPkg}
	case string(KindAPI):
		return []Kind{KindAPI}
	case string(KindSDK):
		return []Kind{KindSDK}
	case string(KindSDKIndex):
		return []Kind{KindSDKIndex}
	case string(KindPartial):
		return []Kind{KindPartial}
	default:
		return []Kind{KindPkg, KindAPI, KindSDK, KindSDKIndex}
	}
}

func (s Service) cacheMirrorStatus() CacheMirrorStatus {
	status := CacheMirrorStatus{Timeout: 5, Fallback: true}
	if s.Config == nil {
		return status
	}
	status.URL = util.DerefString(s.Config.CacheMirror.URL)
	if s.Config.CacheMirror.Enable != nil {
		status.Enable = *s.Config.CacheMirror.Enable
	}
	if s.Config.CacheMirror.Timeout != nil && *s.Config.CacheMirror.Timeout > 0 {
		status.Timeout = *s.Config.CacheMirror.Timeout
	}
	if s.Config.CacheMirror.Fallback != nil {
		status.Fallback = *s.Config.CacheMirror.Fallback
	}
	return status
}
