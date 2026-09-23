package cache

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	"github.com/inherelab/eget/internal/cachemirror"
)

func TestManifestHandlerIndexesCacheFiles(t *testing.T) {
	cacheDir := t.TempDir()
	file := filepath.Join(cacheDir, "pkg.zip")
	assert.NoErr(t, os.WriteFile(file, []byte("pkg"), 0o644))
	assert.NoErr(t, os.WriteFile(filepath.Join(cacheDir, "pkg.zip.part"), []byte("partial"), 0o644))
	fixed := time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC)
	service := Service{Now: func() time.Time { return fixed }}
	req := httptest.NewRequest(http.MethodGet, "/manifest.json", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()

	ManifestHandler(service, cacheDir, MachineOptions{})(rec, req)

	assert.Eq(t, http.StatusOK, rec.Code)
	var manifest Manifest
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &manifest))
	assert.Eq(t, 1, manifest.Schema)
	assert.Eq(t, "eget-cache", manifest.Server.Name)
	assert.Eq(t, "", manifest.Cache.Root)
	assert.Eq(t, 1, len(manifest.Files))
	assert.Eq(t, "pkg", manifest.Files[0].Kind)
	assert.Eq(t, "pkg.zip", manifest.Files[0].Path)
	assert.Eq(t, "path-md5:7d666be70f6586be664607040ebc2977", manifest.Files[0].PathKey)
	assert.Eq(t, "/files/pkg.zip", manifest.Files[0].URL)
	assert.Eq(t, "http://example.com", manifest.Server.BaseURL)
}

func TestFileHandlerServesDownloadHeadAndRange(t *testing.T) {
	cacheDir := t.TempDir()
	file := filepath.Join(cacheDir, "sdk-downloads", "go", "1.22.0", "go.zip")
	assert.NoErr(t, os.MkdirAll(filepath.Dir(file), 0o755))
	assert.NoErr(t, os.WriteFile(file, []byte("0123456789"), 0o644))
	handler := FileHandler(Service{}, cacheDir, MachineOptions{})

	getRec := httptest.NewRecorder()
	handler(getRec, httptest.NewRequest(http.MethodGet, "/files/sdk-downloads/go/1.22.0/go.zip", nil))
	assert.Eq(t, http.StatusOK, getRec.Code)
	assert.Eq(t, "0123456789", getRec.Body.String())

	headRec := httptest.NewRecorder()
	handler(headRec, httptest.NewRequest(http.MethodHead, "/files/sdk-downloads/go/1.22.0/go.zip", nil))
	assert.Eq(t, http.StatusOK, headRec.Code)
	assert.Eq(t, "", headRec.Body.String())

	rangeReq := httptest.NewRequest(http.MethodGet, "/files/sdk-downloads/go/1.22.0/go.zip", nil)
	rangeReq.Header.Set("Range", "bytes=2-5")
	rangeRec := httptest.NewRecorder()
	handler(rangeRec, rangeReq)
	assert.Eq(t, http.StatusPartialContent, rangeRec.Code)
	assert.Eq(t, "2345", rangeRec.Body.String())
}

func TestFileHandlerRejectsPathEscape(t *testing.T) {
	cacheDir := t.TempDir()
	rec := httptest.NewRecorder()

	FileHandler(Service{}, cacheDir, MachineOptions{})(rec, httptest.NewRequest(http.MethodGet, "/files/../secret.txt", nil))

	assert.Eq(t, http.StatusForbidden, rec.Code)
}

func TestFileHandlerNoIndexRejectsDirectoryListing(t *testing.T) {
	cacheDir := t.TempDir()
	assert.NoErr(t, os.MkdirAll(filepath.Join(cacheDir, "sdk-downloads"), 0o755))
	rec := httptest.NewRecorder()

	FileHandler(Service{}, cacheDir, MachineOptions{NoIndex: true})(rec, httptest.NewRequest(http.MethodGet, "/files/sdk-downloads/", nil))

	assert.Eq(t, http.StatusForbidden, rec.Code)
}

func TestManifestHandlerRootScopeFiltersFiles(t *testing.T) {
	cacheDir := t.TempDir()
	assert.NoErr(t, os.WriteFile(filepath.Join(cacheDir, "pkg.zip"), []byte("pkg"), 0o644))
	assert.NoErr(t, os.MkdirAll(filepath.Join(cacheDir, "sdk-downloads"), 0o755))
	assert.NoErr(t, os.WriteFile(filepath.Join(cacheDir, "sdk-downloads", "go.zip"), []byte("sdk"), 0o644))
	rec := httptest.NewRecorder()

	ManifestHandler(Service{}, cacheDir, MachineOptions{Root: "sdk"})(rec, httptest.NewRequest(http.MethodGet, "/manifest.json", nil))

	assert.Eq(t, http.StatusOK, rec.Code)
	var manifest Manifest
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &manifest))
	assert.Eq(t, 1, len(manifest.Files))
	assert.Eq(t, "sdk", manifest.Files[0].Kind)
}

func TestFileHandlerRootScopeRejectsFileOutsideScope(t *testing.T) {
	cacheDir := t.TempDir()
	assert.NoErr(t, os.WriteFile(filepath.Join(cacheDir, "pkg.zip"), []byte("pkg"), 0o644))
	rec := httptest.NewRecorder()

	FileHandler(Service{}, cacheDir, MachineOptions{Root: "sdk"})(rec, httptest.NewRequest(http.MethodGet, "/files/pkg.zip", nil))

	assert.Eq(t, http.StatusForbidden, rec.Code)
}

func TestFileHandlerRejectsPartialFiles(t *testing.T) {
	cacheDir := t.TempDir()
	assert.NoErr(t, os.WriteFile(filepath.Join(cacheDir, "pkg.zip.part"), []byte("partial"), 0o644))
	rec := httptest.NewRecorder()

	FileHandler(Service{}, cacheDir, MachineOptions{})(rec, httptest.NewRequest(http.MethodGet, "/files/pkg.zip.part", nil))

	assert.Eq(t, http.StatusForbidden, rec.Code)
}

func TestFileHandlerRejectsSymlinkEscape(t *testing.T) {
	cacheDir := t.TempDir()
	outsideFile := filepath.Join(t.TempDir(), "secret.txt")
	assert.NoErr(t, os.WriteFile(outsideFile, []byte("secret"), 0o644))
	link := filepath.Join(cacheDir, "sdk-downloads", "leak")
	assert.NoErr(t, os.MkdirAll(filepath.Dir(link), 0o755))
	if err := os.Symlink(outsideFile, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	rec := httptest.NewRecorder()

	FileHandler(Service{}, cacheDir, MachineOptions{})(rec, httptest.NewRequest(http.MethodGet, "/files/sdk-downloads/leak", nil))

	assert.Eq(t, http.StatusForbidden, rec.Code)
}

func TestManifestHandlerExcludesSymlinkEscape(t *testing.T) {
	cacheDir := t.TempDir()
	assert.NoErr(t, os.MkdirAll(filepath.Join(cacheDir, "sdk-downloads"), 0o755))
	outsideFile := filepath.Join(t.TempDir(), "secret.zip")
	assert.NoErr(t, os.WriteFile(outsideFile, []byte("secret"), 0o644))
	link := filepath.Join(cacheDir, "sdk-downloads", "secret.zip")
	if err := os.Symlink(outsideFile, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	rec := httptest.NewRecorder()

	ManifestHandler(Service{}, cacheDir, MachineOptions{})(rec, httptest.NewRequest(http.MethodGet, "/manifest.json", nil))

	assert.Eq(t, http.StatusOK, rec.Code)
	var manifest Manifest
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &manifest))
	assert.Eq(t, 0, len(manifest.Files))
}

func TestDownloadHandlerServesPathKey(t *testing.T) {
	cacheDir := t.TempDir()
	file := filepath.Join(cacheDir, "pkg-cache", "tool.zip")
	assert.NoErr(t, os.MkdirAll(filepath.Dir(file), 0o755))
	assert.NoErr(t, os.WriteFile(file, []byte("pkg"), 0o644))
	key := cachemirror.KeyForRelPath("pkg-cache/tool.zip")
	rec := httptest.NewRecorder()

	DownloadHandler(Service{}, cacheDir, MachineOptions{})(rec, httptest.NewRequest(http.MethodGet, "/download/"+key, nil))

	assert.Eq(t, http.StatusOK, rec.Code)
	assert.Eq(t, "pkg", rec.Body.String())
}

func TestDownloadHandlerServesAPICache(t *testing.T) {
	cacheDir := t.TempDir()
	rel := filepath.ToSlash(filepath.Join("api-cache", "github-repos-owner-tool-releases-latest.json"))
	file := filepath.Join(cacheDir, filepath.FromSlash(rel))
	assert.NoErr(t, os.MkdirAll(filepath.Dir(file), 0o755))
	assert.NoErr(t, os.WriteFile(file, []byte(`{"tag_name":"v1.2.3"}`), 0o644))
	rec := httptest.NewRecorder()

	DownloadHandler(Service{}, cacheDir, MachineOptions{})(rec, httptest.NewRequest(http.MethodGet, "/download/"+cachemirror.KeyForRelPath(rel), nil))

	assert.Eq(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"tag_name":"v1.2.3"`)
}

func TestDownloadHandlerPathKeyMiss(t *testing.T) {
	cacheDir := t.TempDir()
	rec := httptest.NewRecorder()

	DownloadHandler(Service{}, cacheDir, MachineOptions{})(rec, httptest.NewRequest(http.MethodGet, "/download/path-md5:missing", nil))

	assert.Eq(t, http.StatusNotFound, rec.Code)
}

func TestDownloadHandlerRespectsRootScope(t *testing.T) {
	cacheDir := t.TempDir()
	assert.NoErr(t, os.WriteFile(filepath.Join(cacheDir, "pkg.zip"), []byte("pkg"), 0o644))
	key := cachemirror.KeyForRelPath("pkg.zip")
	rec := httptest.NewRecorder()

	DownloadHandler(Service{}, cacheDir, MachineOptions{Root: "sdk"})(rec, httptest.NewRequest(http.MethodGet, "/download/"+key, nil))

	assert.Eq(t, http.StatusNotFound, rec.Code)
}

func TestDownloadHandlerRejectsPartial(t *testing.T) {
	cacheDir := t.TempDir()
	assert.NoErr(t, os.WriteFile(filepath.Join(cacheDir, "pkg.zip.part"), []byte("partial"), 0o644))
	key := cachemirror.KeyForRelPath("pkg.zip.part")
	rec := httptest.NewRecorder()

	DownloadHandler(Service{}, cacheDir, MachineOptions{})(rec, httptest.NewRequest(http.MethodGet, "/download/"+key, nil))

	assert.Eq(t, http.StatusNotFound, rec.Code)
}
