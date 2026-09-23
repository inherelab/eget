package cache

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/inherelab/eget/internal/cachemirror"
)

// Manifest is the JSON cache index eget clients use as a cache mirror source.
type Manifest struct {
	Schema int            `json:"schema"`
	Server ManifestServer `json:"server"`
	Cache  ManifestCache  `json:"cache"`
	Files  []ManifestFile `json:"files"`
}

type ManifestServer struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	BaseURL string `json:"base_url"`
}

type ManifestCache struct {
	Root        string    `json:"root"`
	GeneratedAt time.Time `json:"generated_at"`
}

type ManifestFile struct {
	Kind    string    `json:"kind"`
	Path    string    `json:"path"`
	PathKey string    `json:"path_key,omitempty"`
	URL     string    `json:"url"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// MachineOptions configures the machine endpoints (/manifest.json, /download/*,
// /files/*). Authentication is the caller's job: `eget web` wraps these
// handlers with its own token middleware, so no token lives here.
type MachineOptions struct {
	Root    string
	NoIndex bool
	Version string
}

type machineHandler struct {
	service  Service
	cacheDir string
	opts     MachineOptions
}

func newMachineHandler(service Service, cacheDir string, opts MachineOptions) machineHandler {
	if opts.Root == "" {
		opts.Root = "all"
	}
	return machineHandler{service: service, cacheDir: cacheDir, opts: opts}
}

// ManifestHandler serves the cache manifest at /manifest.json.
func ManifestHandler(service Service, cacheDir string, opts MachineOptions) http.HandlerFunc {
	return newMachineHandler(service, cacheDir, opts).manifest
}

// DownloadHandler serves /download/<path-md5 key>, the protocol eget clients
// use to fetch a cached file by content path.
func DownloadHandler(service Service, cacheDir string, opts MachineOptions) http.HandlerFunc {
	return newMachineHandler(service, cacheDir, opts).download
}

// FileHandler serves /files/<relative path> for direct browsing.
func FileHandler(service Service, cacheDir string, opts MachineOptions) http.HandlerFunc {
	return newMachineHandler(service, cacheDir, opts).file
}

func (h machineHandler) manifest(w http.ResponseWriter, r *http.Request) {
	entries, err := h.service.Scan(h.cacheDir, CacheScanOptions{
		Root:  h.opts.Root,
		Kinds: []Kind{KindPkg, KindAPI, KindSDK, KindSDKIndex},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	files := make([]ManifestFile, 0, len(entries))
	for _, entry := range entries {
		if !pathStaysInDirAfterSymlinks(h.cacheDir, entry.Path) {
			continue
		}
		files = append(files, ManifestFile{
			Kind:    string(entry.Kind),
			Path:    entry.RelPath,
			PathKey: cachemirror.KeyForRelPath(entry.RelPath),
			URL:     "/files/" + path.Clean(entry.RelPath),
			Size:    entry.Size,
			ModTime: entry.ModTime,
		})
	}

	writeJSON(w, http.StatusOK, Manifest{
		Schema: 1,
		Server: ManifestServer{
			Name:    "eget-cache",
			Version: h.opts.Version,
			BaseURL: cacheBaseURL(r),
		},
		Cache: ManifestCache{
			Root:        "",
			GeneratedAt: h.service.now(),
		},
		Files: files,
	})
}

func (h machineHandler) download(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key := strings.TrimPrefix(r.URL.Path, "/download/")
	if !strings.HasPrefix(key, cachemirror.PathMD5Prefix) {
		http.NotFound(w, r)
		return
	}

	entries, err := h.service.Scan(h.cacheDir, CacheScanOptions{
		Root:  h.opts.Root,
		Kinds: []Kind{KindPkg, KindAPI, KindSDK, KindSDKIndex},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, entry := range entries {
		if cachemirror.KeyForRelPath(entry.RelPath) != key {
			continue
		}
		if !pathStaysInDirAfterSymlinks(h.cacheDir, entry.Path) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.ServeFile(w, r, entry.Path)
		return
	}
	http.NotFound(w, r)
}

func (h machineHandler) file(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rel := strings.TrimPrefix(r.URL.Path, "/files/")
	cleanRel, err := cleanCacheRelPath(rel)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	kind, partial := classifyEntry(cleanRel)
	if partial || !cacheRootAllows(h.opts.Root, kind) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	fullPath := filepath.Join(h.cacheDir, filepath.FromSlash(cleanRel))
	if err := ensurePathInDir(h.cacheDir, fullPath); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	realRoot, err := filepath.EvalSymlinks(h.cacheDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	realPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := ensurePathInDir(realRoot, realPath); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	info, err := os.Stat(realPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if info.IsDir() && h.opts.NoIndex {
		http.Error(w, "directory listing disabled", http.StatusForbidden)
		return
	}

	http.ServeFile(w, r, realPath)
}

func pathStaysInDirAfterSymlinks(root, target string) bool {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	realTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return false
	}
	return ensurePathInDir(realRoot, realTarget) == nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func cacheBaseURL(r *http.Request) string {
	host := r.Host
	if host == "" {
		host = r.URL.Host
	}
	if host == "" {
		return ""
	}
	return "http://" + host
}

func cleanCacheRelPath(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("empty path")
	}
	rel = strings.ReplaceAll(rel, "\\", "/")
	for _, part := range strings.Split(rel, "/") {
		if part == ".." {
			return "", fmt.Errorf("invalid path")
		}
	}
	clean := path.Clean("/" + rel)
	clean = strings.TrimPrefix(clean, "/")
	if clean == "." || clean == "" || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", fmt.Errorf("invalid path")
	}
	if strings.Contains(clean, "/../") {
		return "", fmt.Errorf("invalid path")
	}
	return clean, nil
}
