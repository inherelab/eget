# SourceForge Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add direct and managed SourceForge project support through `sourceforge:<project>` targets, optional `source_path`, install/download reuse, and update/outdated checks.

**Architecture:** Add SourceForge as a peer source backend under `internal/source/sourceforge`, then wire it into the existing `install.Finder` selection path. Keep asset selection, download, verification, extraction, and installed-store recording in the existing install runner; SourceForge only discovers candidate URLs and latest versions.

**Tech Stack:** Go 1.24.2, existing `net/http` getter interfaces, existing config/installed TOML model, existing install detector chain, `go test ./...`.

---

## File Structure

- Create `internal/source/sourceforge/target.go`: parse and normalize `sourceforge:<project>` and `sourceforge:<project>/<path>` targets.
- Create `internal/source/sourceforge/files.go`: fetch SourceForge file pages and parse embedded `net.sf.files = {...};` JSON.
- Create `internal/source/sourceforge/finder.go`: implement `Find()` and latest-version discovery from parsed file entries.
- Create `internal/source/sourceforge/version.go`: extract and compare version-like strings.
- Modify `internal/install/options.go`: add `SourcePath` to `Options`, add `TargetSourceForge`, detect SourceForge targets.
- Modify `internal/install/service.go`: route SourceForge targets to the new finder and detect source path conflicts.
- Modify `internal/config/model.go` and `internal/config/merge.go`: add `source_path`.
- Modify `internal/app/config.go`: persist `source_path` when adding SourceForge packages.
- Modify `internal/app/install.go`: record SourceForge normalized repo and version in installed store.
- Modify `internal/app/list.go` and `internal/app/update.go`: pass `source_path` into latest-version checks.
- Modify `internal/cli/wiring.go`: wire SourceForge finder/latest checker with existing network options.
- Modify docs: `README.md`, `README.zh-CN.md`, `docs/DOCS.md`, `docs/example.eget.toml`.

## Constraints

- Do not add SourceForge-specific asset matching to `internal/install/runner.go`.
- Do not fall back to old installed assets when current SourceForge assets do not match.
- Do not add SourceForge search or `query sourceforge:...` in this plan.
- Prefer config support for `source_path`; do not add a CLI `--source-path` flag in the first implementation.
- Commit after each task.

---

### Task 1: Add SourceForge Target Parsing

**Files:**
- Create: `internal/source/sourceforge/target.go`
- Create: `internal/source/sourceforge/target_test.go`
- Modify: `internal/install/options.go`
- Test: `internal/install/options_test.go`

- [x] **Step 1: Write SourceForge target parser tests**

Add `internal/source/sourceforge/target_test.go`:

```go
package sourceforge

import (
	"strings"
	"testing"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantProject string
		wantPath    string
		wantNorm    string
		wantErr     string
	}{
		{name: "project only", input: "sourceforge:winmerge", wantProject: "winmerge", wantNorm: "sourceforge:winmerge"},
		{name: "project with path", input: "sourceforge:winmerge/stable", wantProject: "winmerge", wantPath: "stable", wantNorm: "sourceforge:winmerge"},
		{name: "nested path", input: "sourceforge:winmerge/stable/2.16.44", wantProject: "winmerge", wantPath: "stable/2.16.44", wantNorm: "sourceforge:winmerge"},
		{name: "empty project", input: "sourceforge:", wantErr: "sourceforge project is required"},
		{name: "not sourceforge", input: "junegunn/fzf", wantErr: "invalid SourceForge target"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTarget(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTarget(): %v", err)
			}
			if got.Project != tt.wantProject || got.Path != tt.wantPath || got.Normalized != tt.wantNorm {
				t.Fatalf("unexpected target: %#v", got)
			}
		})
	}
}
```

- [x] **Step 2: Run parser test to verify it fails**

Run: `go test ./internal/source/sourceforge -run TestParseTarget -v`

Expected: FAIL because the package or `ParseTarget` does not exist.

- [x] **Step 3: Implement minimal target parser**

Create `internal/source/sourceforge/target.go`:

```go
package sourceforge

import (
	"fmt"
	"strings"
)

const Prefix = "sourceforge:"

type Target struct {
	Project    string
	Path       string
	Normalized string
}

func IsTarget(value string) bool {
	return strings.HasPrefix(value, Prefix)
}

func ParseTarget(value string) (Target, error) {
	if !IsTarget(value) {
		return Target{}, fmt.Errorf("invalid SourceForge target %q", value)
	}
	rest := strings.TrimPrefix(value, Prefix)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return Target{}, fmt.Errorf("sourceforge project is required")
	}
	project, sourcePath, _ := strings.Cut(rest, "/")
	if project == "" {
		return Target{}, fmt.Errorf("sourceforge project is required")
	}
	sourcePath = strings.Trim(sourcePath, "/")
	return Target{
		Project:    project,
		Path:       sourcePath,
		Normalized: Prefix + project,
	}, nil
}
```

- [x] **Step 4: Add install target kind tests**

In `internal/install/options_test.go`, extend `TestDetectTargetKind` cases:

```go
{name: "sourceforge target", target: "sourceforge:winmerge", want: TargetSourceForge},
{name: "sourceforge target with path", target: "sourceforge:winmerge/stable", want: TargetSourceForge},
```

- [x] **Step 5: Implement target kind detection**

In `internal/install/options.go`:

```go
const (
	TargetUnknown     TargetKind = "unknown"
	TargetRepo        TargetKind = "repo"
	TargetGitHubURL   TargetKind = "github_url"
	TargetSourceForge TargetKind = "sourceforge"
	TargetDirectURL   TargetKind = "direct_url"
	TargetLocalFile   TargetKind = "local_file"
)
```

Import `sourceforge`:

```go
sourceforge "github.com/inherelab/eget/internal/source/sourceforge"
```

Update `DetectTargetKind` before URL and repo checks:

```go
case sourceforge.IsTarget(target):
	return TargetSourceForge
```

- [x] **Step 6: Run tests**

Run:

```bash
go test ./internal/source/sourceforge ./internal/install -run 'TestParseTarget|TestDetectTargetKind' -v
```

Expected: PASS.

- [x] **Step 7: Commit**

```bash
git add internal/source/sourceforge/target.go internal/source/sourceforge/target_test.go internal/install/options.go internal/install/options_test.go
git commit -m "feat(sourceforge): detect sourceforge targets"
```

---

### Task 2: Add `source_path` Configuration and Persistence

**Files:**
- Modify: `internal/config/model.go`
- Modify: `internal/config/merge.go`
- Modify: `internal/config/merge_test.go`
- Modify: `internal/install/options.go`
- Modify: `internal/app/config.go`
- Modify: `internal/app/add_test.go`

- [x] **Step 1: Write config merge test**

In `internal/config/merge_test.go`, add:

```go
func TestMergeInstallOptionsMergesSourcePath(t *testing.T) {
	global := Section{SourcePath: stringPtr("global-path")}
	repo := Section{SourcePath: stringPtr("repo-path")}
	pkg := Section{SourcePath: stringPtr("pkg-path")}
	cli := CLIOverrides{SourcePath: stringPtr("cli-path")}

	merged := MergeInstallOptions(global, repo, pkg, cli)
	assert.Eq(t, "cli-path", merged.SourcePath)

	merged = MergeInstallOptions(global, repo, pkg, CLIOverrides{})
	assert.Eq(t, "pkg-path", merged.SourcePath)

	merged = MergeInstallOptions(global, repo, Section{}, CLIOverrides{})
	assert.Eq(t, "repo-path", merged.SourcePath)
}
```

Use the existing helper pattern in that test file for `stringPtr`; if it does not exist, add:

```go
func stringPtr(value string) *string { return &value }
```

- [x] **Step 2: Run config test to verify it fails**

Run: `go test ./internal/config -run TestMergeInstallOptionsMergesSourcePath -v`

Expected: FAIL because `SourcePath` does not exist.

- [x] **Step 3: Add `SourcePath` fields**

In `internal/config/model.go`, add to `Section`:

```go
SourcePath *string `toml:"source_path" mapstructure:"source_path"`
```

Add to `Merged`:

```go
SourcePath string
```

Add to `CLIOverrides`:

```go
SourcePath *string
```

In `internal/config/merge.go`, add:

```go
merged.SourcePath = firstString(cli.SourcePath, pkg.SourcePath, repo.SourcePath, global.SourcePath)
```

- [x] **Step 4: Add install option field and app option merge**

In `internal/install/options.go`, add:

```go
SourcePath string
```

In `internal/app/install.go`, include `SourcePath` in `cfgpkg.CLIOverrides`:

```go
SourcePath: stringOpt(cli.SourcePath),
```

And include it in returned `install.Options`:

```go
SourcePath: merged.SourcePath,
```

- [x] **Step 5: Write add persistence test**

In `internal/app/add_test.go`, extend `TestAddPackage` setup:

```go
opts := install.Options{
	// existing fields...
	SourcePath: "stable",
}
```

Add assertion:

```go
if pkg.SourcePath == nil || *pkg.SourcePath != "stable" {
	t.Fatalf("expected source_path to be persisted, got %#v", pkg.SourcePath)
}
```

- [x] **Step 6: Persist source path from install options**

In `internal/app/config.go`, inside `sectionFromInstallOptions`:

```go
if opts.SourcePath != "" {
	section.SourcePath = util.StringPtr(opts.SourcePath)
}
```

- [x] **Step 7: Run tests**

Run:

```bash
go test ./internal/config ./internal/app -run 'TestMergeInstallOptionsMergesSourcePath|TestAddPackage' -v
```

Expected: PASS.

- [x] **Step 8: Commit**

```bash
git add internal/config/model.go internal/config/merge.go internal/config/merge_test.go internal/install/options.go internal/app/install.go internal/app/config.go internal/app/add_test.go
git commit -m "feat(config): add source path option"
```

---

### Task 3: Normalize SourceForge Targets in Config and Installed Store

**Files:**
- Modify: `internal/app/config.go`
- Modify: `internal/app/add_test.go`
- Modify: `internal/installed/store.go`
- Modify: `internal/installed/store_test.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/update_test.go`

- [x] **Step 1: Write add normalization test**

In `internal/app/add_test.go`, add:

```go
func TestAddPackageNormalizesSourceForgeTargetWithPath(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")
	svc := ConfigService{
		ConfigPath: configPath,
		Load: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		Save: cfgpkg.Save,
	}

	if err := svc.AddPackage("sourceforge:winmerge/stable", "", install.Options{}); err != nil {
		t.Fatalf("add sourceforge package: %v", err)
	}

	cfg, err := cfgpkg.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	pkg, ok := cfg.Packages["winmerge"]
	if !ok {
		t.Fatalf("expected packages.winmerge, got %#v", cfg.Packages)
	}
	if pkg.Repo == nil || *pkg.Repo != "sourceforge:winmerge" {
		t.Fatalf("expected normalized repo, got %#v", pkg.Repo)
	}
	if pkg.SourcePath == nil || *pkg.SourcePath != "stable" {
		t.Fatalf("expected source_path stable, got %#v", pkg.SourcePath)
	}
}
```

- [x] **Step 2: Write installed-store normalization test**

In `internal/installed/store_test.go`, add:

```go
func TestNormalizeRepoNameSourceForge(t *testing.T) {
	assert.Eq(t, "sourceforge:winmerge", NormalizeRepoName("sourceforge:winmerge"))
	assert.Eq(t, "sourceforge:winmerge", NormalizeRepoName("sourceforge:winmerge/stable"))
}
```

- [x] **Step 3: Run tests to verify they fail**

Run:

```bash
go test ./internal/app ./internal/installed -run 'TestAddPackageNormalizesSourceForgeTargetWithPath|TestNormalizeRepoNameSourceForge' -v
```

Expected: FAIL because SourceForge normalization is not implemented.

- [x] **Step 4: Implement config normalization**

In `internal/app/config.go`, import SourceForge package:

```go
sourceforge "github.com/inherelab/eget/internal/source/sourceforge"
```

At the start of `AddPackage`, after loading config:

```go
if sfTarget, sfErr := sourceforge.ParseTarget(repo); sfErr == nil {
	repo = sfTarget.Normalized
	if opts.SourcePath == "" {
		opts.SourcePath = sfTarget.Path
	}
	if name == "" {
		name = sfTarget.Project
	}
}
```

Keep existing GitHub-style name fallback after this block.

- [x] **Step 5: Implement installed normalization**

In `internal/installed/store.go`, import SourceForge package:

```go
sourceforge "github.com/inherelab/eget/internal/source/sourceforge"
```

At the top of `NormalizeRepoName`:

```go
if sfTarget, err := sourceforge.ParseTarget(target); err == nil {
	return sfTarget.Normalized
}
```

- [x] **Step 6: Allow direct SourceForge update target**

In `internal/app/update.go`, import SourceForge and update direct target check:

```go
if strings.Contains(nameOrRepo, "/") || sourceforge.IsTarget(nameOrRepo) {
	return s.Install.InstallTarget(nameOrRepo, cli)
}
```

Add a test in `internal/app/update_test.go`:

```go
func TestUpdatePackageAllowsDirectSourceForgeTarget(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
	}
	if _, err := svc.UpdatePackage("sourceforge:winmerge", install.Options{}); err != nil {
		t.Fatalf("update sourceforge direct target: %v", err)
	}
	assert.Eq(t, []string{"sourceforge:winmerge"}, installer.targets)
}
```

- [x] **Step 7: Run tests**

Run:

```bash
go test ./internal/app ./internal/installed -run 'TestAddPackageNormalizesSourceForgeTargetWithPath|TestNormalizeRepoNameSourceForge|TestUpdatePackageAllowsDirectSourceForgeTarget' -v
```

Expected: PASS.

- [x] **Step 8: Commit**

```bash
git add internal/app/config.go internal/app/add_test.go internal/installed/store.go internal/installed/store_test.go internal/app/update.go internal/app/update_test.go
git commit -m "feat(sourceforge): normalize managed targets"
```

---

### Task 4: Implement SourceForge File Page Parser

**Files:**
- Create: `internal/source/sourceforge/files.go`
- Create: `internal/source/sourceforge/files_test.go`

- [x] **Step 1: Write parser tests**

Create `internal/source/sourceforge/files_test.go`:

```go
package sourceforge

import "testing"

func TestParseFilesPage(t *testing.T) {
	html := `<html><script>
net.sf.files = {"2.16.44":{"name":"2.16.44","path":"/stable","download_url":"https://sourceforge.net/projects/winmerge/files/stable/2.16.44/download","url":"/projects/winmerge/files/stable/2.16.44/","full_path":"stable/2.16.44","type":"d","downloadable":false},"WinMerge-2.16.44-x64-Setup.exe":{"name":"WinMerge-2.16.44-x64-Setup.exe","path":"/stable/2.16.44","download_url":"https://sourceforge.net/projects/winmerge/files/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe/download","url":"/projects/winmerge/files/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe/download","full_path":"stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe","type":"f","downloadable":true}};
net.sf.staging_days = 3;
</script></html>`

	files, err := ParseFilesPage([]byte(html))
	if err != nil {
		t.Fatalf("parse files page: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %#v", files)
	}
	if files[0].Name != "2.16.44" || files[0].Type != TypeDirectory {
		t.Fatalf("unexpected first file: %#v", files[0])
	}
	if files[1].Name != "WinMerge-2.16.44-x64-Setup.exe" || !files[1].Downloadable {
		t.Fatalf("unexpected second file: %#v", files[1])
	}
}

func TestParseFilesPageRejectsMissingData(t *testing.T) {
	_, err := ParseFilesPage([]byte(`<html></html>`))
	if err == nil {
		t.Fatal("expected missing net.sf.files to fail")
	}
}
```

- [x] **Step 2: Run parser tests to verify they fail**

Run: `go test ./internal/source/sourceforge -run TestParseFilesPage -v`

Expected: FAIL because parser does not exist.

- [x] **Step 3: Implement parser**

Create `internal/source/sourceforge/files.go`:

```go
package sourceforge

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
)

type FileType string

const (
	TypeFile      FileType = "f"
	TypeDirectory FileType = "d"
)

type File struct {
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	DownloadURL  string   `json:"download_url"`
	URL          string   `json:"url"`
	FullPath     string   `json:"full_path"`
	Type         FileType `json:"type"`
	Downloadable bool     `json:"downloadable"`
}

var filesObjectPattern = regexp.MustCompile(`(?s)net\.sf\.files\s*=\s*(\{.*?\});`)

func ParseFilesPage(body []byte) ([]File, error) {
	matches := filesObjectPattern.FindSubmatch(body)
	if len(matches) != 2 {
		return nil, fmt.Errorf("sourceforge files data not found")
	}
	byName := map[string]File{}
	if err := json.Unmarshal(matches[1], &byName); err != nil {
		return nil, fmt.Errorf("decode sourceforge files data: %w", err)
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	files := make([]File, 0, len(names))
	for _, name := range names {
		files = append(files, byName[name])
	}
	return files, nil
}
```

- [x] **Step 4: Run parser tests**

Run: `go test ./internal/source/sourceforge -run TestParseFilesPage -v`

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/source/sourceforge/files.go internal/source/sourceforge/files_test.go
git commit -m "feat(sourceforge): parse file listings"
```

---

### Task 5: Implement SourceForge Version Helpers and Finder

**Files:**
- Create: `internal/source/sourceforge/version.go`
- Create: `internal/source/sourceforge/version_test.go`
- Create: `internal/source/sourceforge/finder.go`
- Create: `internal/source/sourceforge/finder_test.go`

- [x] **Step 1: Write version tests**

Create `internal/source/sourceforge/version_test.go`:

```go
package sourceforge

import "testing"

func TestVersionFromText(t *testing.T) {
	tests := map[string]string{
		"2.16.44":                          "2.16.44",
		"WinMerge-2.16.44-x64-Setup.exe":   "2.16.44",
		"winmerge-2.16.44-src.zip":         "2.16.44",
		"stable":                           "",
	}
	for input, want := range tests {
		if got := VersionFromText(input); got != want {
			t.Fatalf("VersionFromText(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLatestVersionFile(t *testing.T) {
	files := []File{
		{Name: "2.16.42", Type: TypeDirectory, FullPath: "stable/2.16.42"},
		{Name: "2.16.44", Type: TypeDirectory, FullPath: "stable/2.16.44"},
		{Name: "readme.txt", Type: TypeFile, FullPath: "stable/readme.txt"},
	}
	got, err := LatestVersionFile(files)
	if err != nil {
		t.Fatalf("latest version file: %v", err)
	}
	if got.Name != "2.16.44" {
		t.Fatalf("expected 2.16.44, got %#v", got)
	}
}
```

- [x] **Step 2: Implement version helpers**

Create `internal/source/sourceforge/version.go`:

```go
package sourceforge

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var versionPattern = regexp.MustCompile(`\d+(?:\.\d+)+`)

func VersionFromText(value string) string {
	return versionPattern.FindString(value)
}

func LatestVersionFile(files []File) (File, error) {
	candidates := make([]File, 0, len(files))
	for _, file := range files {
		if VersionFromText(file.Name) != "" || VersionFromText(file.FullPath) != "" {
			candidates = append(candidates, file)
		}
	}
	if len(candidates) == 0 {
		return File{}, fmt.Errorf("could not determine SourceForge latest version")
	}
	sort.Slice(candidates, func(i, j int) bool {
		return compareVersion(VersionFromText(candidates[i].Name), VersionFromText(candidates[j].Name)) > 0
	})
	return candidates[0], nil
}

func compareVersion(a, b string) int {
	ap := splitVersion(a)
	bp := splitVersion(b)
	max := len(ap)
	if len(bp) > max {
		max = len(bp)
	}
	for i := 0; i < max; i++ {
		av, bv := 0, 0
		if i < len(ap) {
			av = ap[i]
		}
		if i < len(bp) {
			bv = bp[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func splitVersion(value string) []int {
	parts := strings.Split(value, ".")
	nums := make([]int, 0, len(parts))
	for _, part := range parts {
		n, _ := strconv.Atoi(part)
		nums = append(nums, n)
	}
	return nums
}
```

- [x] **Step 3: Write finder tests**

Create `internal/source/sourceforge/finder_test.go`:

```go
package sourceforge

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeGetter func(url string) (*http.Response, error)

func (f fakeGetter) Get(url string) (*http.Response, error) { return f(url) }

func TestFinderFindsLatestFilesUnderSourcePath(t *testing.T) {
	requests := []string{}
	getter := fakeGetter(func(url string) (*http.Response, error) {
		requests = append(requests, url)
		switch {
		case strings.Contains(url, "/projects/winmerge/files/stable/"):
			return htmlResponse(`net.sf.files = {"2.16.42":{"name":"2.16.42","full_path":"stable/2.16.42","download_url":"https://sourceforge.net/projects/winmerge/files/stable/2.16.42/download","type":"d","downloadable":false},"2.16.44":{"name":"2.16.44","full_path":"stable/2.16.44","download_url":"https://sourceforge.net/projects/winmerge/files/stable/2.16.44/download","type":"d","downloadable":false}};`), nil
		case strings.Contains(url, "/projects/winmerge/files/stable/2.16.44/"):
			return htmlResponse(`net.sf.files = {"WinMerge-2.16.44-x64-Setup.exe":{"name":"WinMerge-2.16.44-x64-Setup.exe","full_path":"stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe","download_url":"https://sourceforge.net/projects/winmerge/files/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe/download","type":"f","downloadable":true}};`), nil
		default:
			t.Fatalf("unexpected request %q", url)
			return nil, nil
		}
	})
	finder := Finder{Project: "winmerge", Path: "stable", Getter: getter}
	assets, err := finder.Find()
	if err != nil {
		t.Fatalf("Find(): %v", err)
	}
	if len(assets) != 1 || !strings.Contains(assets[0], "WinMerge-2.16.44-x64-Setup.exe/download") {
		t.Fatalf("unexpected assets: %#v", assets)
	}
	if len(requests) != 2 {
		t.Fatalf("expected two listing requests, got %#v", requests)
	}
}

func htmlResponse(script string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(`<html><script>` + script + `</script></html>`)),
	}
}
```

- [x] **Step 4: Implement finder**

Create `internal/source/sourceforge/finder.go`:

```go
package sourceforge

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type HTTPGetter interface {
	Get(url string) (*http.Response, error)
}

type Finder struct {
	Project string
	Path    string
	Tag     string
	Getter  HTTPGetter
}

func (f Finder) Find() ([]string, error) {
	if f.Getter == nil {
		return nil, fmt.Errorf("sourceforge getter is required")
	}
	if f.Project == "" {
		return nil, fmt.Errorf("sourceforge project is required")
	}
	sourcePath := strings.Trim(f.Path, "/")
	if f.Tag != "" {
		sourcePath = path.Join(sourcePath, f.Tag)
	}
	files, err := f.list(sourcePath)
	if err != nil {
		return nil, err
	}
	if hasDownloadable(files) {
		return downloadableURLs(files), nil
	}
	latest, err := LatestVersionFile(files)
	if err != nil {
		return nil, fmt.Errorf("could not determine SourceForge latest version for %s: %w", f.Project, err)
	}
	files, err = f.list(latest.FullPath)
	if err != nil {
		return nil, err
	}
	if !hasDownloadable(files) {
		return nil, fmt.Errorf("no SourceForge files found for %s/%s", f.Project, latest.FullPath)
	}
	return downloadableURLs(files), nil
}

func (f Finder) list(sourcePath string) ([]File, error) {
	u := fmt.Sprintf("https://sourceforge.net/projects/%s/files/", url.PathEscape(f.Project))
	if sourcePath != "" {
		u += strings.Trim(sourcePath, "/") + "/"
	}
	resp, err := f.Getter.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sourceforge files request failed: %s (URL: %s)", resp.Status, u)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return ParseFilesPage(body)
}

func hasDownloadable(files []File) bool {
	return len(downloadableURLs(files)) > 0
}

func downloadableURLs(files []File) []string {
	urls := make([]string, 0, len(files))
	for _, file := range files {
		if file.Type == TypeFile && file.DownloadURL != "" {
			urls = append(urls, file.DownloadURL)
		}
	}
	return urls
}
```

- [x] **Step 5: Run SourceForge package tests**

Run: `go test ./internal/source/sourceforge -v`

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add internal/source/sourceforge/version.go internal/source/sourceforge/version_test.go internal/source/sourceforge/finder.go internal/source/sourceforge/finder_test.go
git commit -m "feat(sourceforge): discover release assets"
```

---

### Task 6: Wire SourceForge Finder Into Install Flow

**Files:**
- Modify: `internal/install/service.go`
- Modify: `internal/install/service_test.go`
- Modify: `internal/cli/wiring.go`
- Modify: `internal/cli/service_test.go`

- [ ] **Step 1: Write install service selection test**

In `internal/install/service_test.go`, add:

```go
func TestSelectFinderSourceForgeTarget(t *testing.T) {
	svc := NewDefaultService(nil, nil)
	opts := &Options{SourcePath: "stable"}
	finder, tool, err := svc.SelectFinder("sourceforge:winmerge", opts)
	if err != nil {
		t.Fatalf("SelectFinder(sourceforge): %v", err)
	}
	if tool != "winmerge" {
		t.Fatalf("expected tool winmerge, got %q", tool)
	}
	got, ok := finder.(interface{ Find() ([]string, error) })
	if !ok || got == nil {
		t.Fatalf("expected sourceforge finder, got %T", finder)
	}
}

func TestSelectFinderRejectsConflictingSourceForgePaths(t *testing.T) {
	svc := NewDefaultService(nil, nil)
	_, _, err := svc.SelectFinder("sourceforge:winmerge/beta", &Options{SourcePath: "stable"})
	if err == nil || !strings.Contains(err.Error(), "source_path") {
		t.Fatalf("expected source_path conflict, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/install -run 'TestSelectFinderSourceForgeTarget|TestSelectFinderRejectsConflictingSourceForgePaths' -v`

Expected: FAIL because SourceForge selection is not wired.

- [ ] **Step 3: Add SourceForge getter factory to install service**

In `internal/install/service.go`, import SourceForge:

```go
sourcesf "github.com/inherelab/eget/internal/source/sourceforge"
```

Add field to `Service`:

```go
SourceForgeGetterFactory func(opts Options) sourcesf.HTTPGetter
```

In `NewDefaultService` in `internal/install/defaults.go`, add:

```go
SourceForgeGetterFactory: func(opts Options) sourcesf.HTTPGetter {
	return NewHTTPGetter(opts)
},
```

- [ ] **Step 4: Route SourceForge target in `SelectFinder`**

In `internal/install/service.go`, add a case:

```go
case TargetSourceForge:
	sfTarget, err := sourcesf.ParseTarget(target)
	if err != nil {
		return nil, "", err
	}
	sourcePath := opts.SourcePath
	if sourcePath != "" && sfTarget.Path != "" && sourcePath != sfTarget.Path {
		return nil, "", fmt.Errorf("source_path %q conflicts with target path %q", sourcePath, sfTarget.Path)
	}
	if sourcePath == "" {
		sourcePath = sfTarget.Path
	}
	getter := s.SourceForgeGetterFactory
	if getter == nil {
		return nil, "", fmt.Errorf("sourceforge getter factory is required")
	}
	return sourcesf.Finder{
		Project: sfTarget.Project,
		Path:    sourcePath,
		Tag:     opts.Tag,
		Getter:  getter(*opts),
	}, sfTarget.Project, nil
```

- [ ] **Step 5: Wire CLI service SourceForge verbose configuration**

In `internal/cli/wiring.go`, import SourceForge package:

```go
sourcesf "github.com/inherelab/eget/internal/source/sourceforge"
```

No special factory is needed if `NewDefaultService` uses `NewHTTPGetter(opts)`. In `configureVerbose`, add:

```go
sourcesf.SetVerbose(verbose, stderr)
```

If `sourceforge.SetVerbose` does not exist, add it in `internal/source/sourceforge/files.go` or a new `log.go`:

```go
var verboseWriter io.Writer = io.Discard
var verboseEnabled bool

func SetVerbose(enabled bool, writer io.Writer) {
	verboseEnabled = enabled
	if writer == nil {
		verboseWriter = io.Discard
		return
	}
	verboseWriter = writer
}
```

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/install ./internal/cli -run 'TestSelectFinderSourceForgeTarget|TestSelectFinderRejectsConflictingSourceForgePaths|TestConfigureVerboseUpdatesVerboseLoggers' -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/install/service.go internal/install/service_test.go internal/install/defaults.go internal/cli/wiring.go internal/source/sourceforge
git commit -m "feat(sourceforge): wire install finder"
```

---

### Task 7: Record SourceForge Versions and Support Latest Checks

**Files:**
- Modify: `internal/source/sourceforge/finder.go`
- Modify: `internal/source/sourceforge/finder_test.go`
- Modify: `internal/app/install.go`
- Modify: `internal/app/install_test.go`
- Modify: `internal/app/list.go`
- Modify: `internal/app/list_test.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/update_test.go`
- Modify: `internal/cli/wiring.go`

- [ ] **Step 1: Add latest info API to SourceForge package**

In `internal/source/sourceforge/finder_test.go`, add:

```go
func TestLatestVersionUsesSourcePath(t *testing.T) {
	getter := fakeGetter(func(url string) (*http.Response, error) {
		if !strings.Contains(url, "/projects/winmerge/files/stable/") {
			t.Fatalf("unexpected request %q", url)
		}
		return htmlResponse(`net.sf.files = {"2.16.42":{"name":"2.16.42","full_path":"stable/2.16.42","download_url":"https://sourceforge.net/projects/winmerge/files/stable/2.16.42/download","type":"d","downloadable":false},"2.16.44":{"name":"2.16.44","full_path":"stable/2.16.44","download_url":"https://sourceforge.net/projects/winmerge/files/stable/2.16.44/download","type":"d","downloadable":false}};`), nil
	})
	info, err := LatestVersion("winmerge", "stable", getter)
	if err != nil {
		t.Fatalf("LatestVersion(): %v", err)
	}
	if info.Version != "2.16.44" || info.Path != "stable/2.16.44" {
		t.Fatalf("unexpected latest info: %#v", info)
	}
}
```

Implement in `internal/source/sourceforge/finder.go`:

```go
type LatestInfo struct {
	Version string
	Path    string
}

func LatestVersion(project, sourcePath string, getter HTTPGetter) (LatestInfo, error) {
	finder := Finder{Project: project, Path: sourcePath, Getter: getter}
	files, err := finder.list(strings.Trim(sourcePath, "/"))
	if err != nil {
		return LatestInfo{}, err
	}
	latest, err := LatestVersionFile(files)
	if err != nil {
		return LatestInfo{}, fmt.Errorf("could not determine SourceForge latest version for %s: %w", project, err)
	}
	version := VersionFromText(latest.Name)
	if version == "" {
		version = VersionFromText(latest.FullPath)
	}
	if version == "" {
		return LatestInfo{}, fmt.Errorf("could not determine SourceForge latest version for %s", project)
	}
	return LatestInfo{Version: version, Path: latest.FullPath}, nil
}
```

- [ ] **Step 2: Record SourceForge version from URL**

In `internal/app/install_test.go`, add:

```go
func TestInstallTargetRecordsSourceForgeVersionFromURL(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://sourceforge.net/projects/winmerge/files/stable/2.16.44/WinMerge-2.16.44-x64-Setup.exe/download",
			Tool:           "winmerge",
			ExtractedFiles: []string{"./WinMerge.exe"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{Runner: runner, Store: store}
	_, err := svc.InstallTarget("sourceforge:winmerge", install.Options{SourcePath: "stable"})
	if err != nil {
		t.Fatalf("install sourceforge target: %v", err)
	}
	if store.entry.Repo != "sourceforge:winmerge" {
		t.Fatalf("expected normalized sourceforge repo, got %q", store.entry.Repo)
	}
	if store.entry.Tag != "2.16.44" || store.entry.Version != "2.16.44" {
		t.Fatalf("expected sourceforge version 2.16.44, got tag=%q version=%q", store.entry.Tag, store.entry.Version)
	}
}
```

In `internal/app/install.go`, when building installed entry, after `tagFromReleaseURL`, add sourceforge parsing:

```go
if tag == "" && strings.HasPrefix(repo, "sourceforge:") {
	tag = sourcesf.VersionFromText(result.URL)
}
```

If tag was found for SourceForge, also set `Version: tag` in `storepkg.Entry`.

- [ ] **Step 3: Pass source path to list items**

In `internal/app/list.go`, add `SourcePath string` to `ListItem` and set it from package config:

```go
SourcePath: util.DerefString(pkg.SourcePath),
```

Change `LatestTag` signature:

```go
LatestTag func(repo, sourcePath string) (string, error)
```

Call it as:

```go
latestTag, err := s.LatestTag(item.Repo, item.SourcePath)
```

Update all list tests' fake functions to accept the second argument.

- [ ] **Step 4: Update update candidate latest signature**

In `internal/app/update.go`, change:

```go
LatestTag func(repo, sourcePath string) (string, error)
```

And call:

```go
latestTag, err := s.LatestTag(item.Repo, item.SourcePath)
```

Update `UpdateService` tests accordingly.

- [ ] **Step 5: Wire source-aware latest checker**

In `internal/cli/wiring.go`, define a helper closure:

```go
latestTag := func(repo, sourcePath string) (string, error) {
	if sfTarget, err := sourcesf.ParseTarget(repo); err == nil {
		info, err := sourcesf.LatestVersion(sfTarget.Project, sourcePath, install.NewHTTPGetter(defaultOpts))
		if err != nil {
			return "", err
		}
		return info.Version, nil
	}
	tag, _, err := githubClient.LatestReleaseInfo(repo)
	return tag, err
}
```

Use it for both `listService.LatestTag` and `updService.LatestTag`.

- [ ] **Step 6: Add mixed outdated test**

In `internal/app/list_test.go`, add a test with GitHub and SourceForge installed packages:

```go
func TestListOutdatedPackagesPassesSourcePathToLatestChecker(t *testing.T) {
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["winmerge"] = cfgpkg.Section{
				Repo:       util.StringPtr("sourceforge:winmerge"),
				SourcePath: util.StringPtr("stable"),
			}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"sourceforge:winmerge": {Repo: "sourceforge:winmerge", Tag: "2.16.42"},
			}}, nil
		},
		LatestTag: func(repo, sourcePath string) (string, error) {
			if repo != "sourceforge:winmerge" || sourcePath != "stable" {
				t.Fatalf("unexpected latest check repo=%q sourcePath=%q", repo, sourcePath)
			}
			return "2.16.44", nil
		},
	}
	items, failures, checked, err := svc.ListOutdatedPackages()
	assert.NoErr(t, err)
	assert.Eq(t, 1, checked)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, "2.16.44", items[0].LatestTag)
}
```

- [ ] **Step 7: Run tests**

Run:

```bash
go test ./internal/source/sourceforge ./internal/app ./internal/cli -run 'SourceForge|Outdated|UpdateAll|UpdatePackage' -v
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/source/sourceforge internal/app/install.go internal/app/install_test.go internal/app/list.go internal/app/list_test.go internal/app/update.go internal/app/update_test.go internal/cli/wiring.go internal/cli/service_test.go
git commit -m "feat(sourceforge): support outdated checks"
```

---

### Task 8: Documentation and Example Config

**Files:**
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `docs/DOCS.md`
- Modify: `docs/example.eget.toml`

- [ ] **Step 1: Update README command examples**

In `README.md`, add a SourceForge example near install/add usage:

```markdown
# install a SourceForge project directly
eget install sourceforge:winmerge --asset x64,setup

# add a SourceForge project as a managed package
eget add sourceforge:winmerge --name winmerge --system windows/amd64 --asset x64,setup
```

In the config example section, add:

```toml
[packages.winmerge]
repo = "sourceforge:winmerge"
source_path = "stable"
system = "windows/amd64"
asset_filters = ["x64", "setup"]
```

- [ ] **Step 2: Update Chinese README**

In `README.zh-CN.md`, add equivalent examples:

```markdown
# 直接安装 SourceForge 项目
eget install sourceforge:winmerge --asset x64,setup

# 添加 SourceForge 项目为托管包
eget add sourceforge:winmerge --name winmerge --system windows/amd64 --asset x64,setup
```

Add the same TOML example with Chinese explanatory text.

- [ ] **Step 3: Update docs**

In `docs/DOCS.md`, update runtime layout:

```markdown
- `internal/source/sourceforge`: SourceForge file discovery and latest-version checks
```

Update install target list:

```markdown
- SourceForge target, for example `sourceforge:winmerge`
```

Add a SourceForge section:

```markdown
## SourceForge Flow

`sourceforge:<project>` targets are resolved by `internal/source/sourceforge`.
The optional `source_path` config narrows discovery under the project's files area.
After SourceForge returns candidate download URLs, `system`, `asset_filters`, `file`,
download, verification, extraction, and installed-store recording reuse the normal install flow.
```

- [ ] **Step 4: Update example config**

In `docs/example.eget.toml`, add:

```toml
[packages."winmerge"]
name = "winmerge"
repo = "sourceforge:winmerge"
source_path = "stable"
system = "windows/amd64"
asset_filters = ["x64", "setup"]
is_gui = true
```

- [ ] **Step 5: Commit docs**

```bash
git add README.md README.zh-CN.md docs/DOCS.md docs/example.eget.toml
git commit -m "docs: document sourceforge packages"
```

---

### Task 9: Full Verification and Manual Smoke Test

**Files:**
- No required code files.
- Optional: update this checklist as tasks complete.

- [ ] **Step 1: Run full tests**

Run:

```bash
go test ./...
```

Expected: all packages PASS.

- [ ] **Step 2: Build binary**

Run:

```bash
go build ./cmd/eget
```

Expected: command exits successfully and creates `eget.exe` on Windows.

- [ ] **Step 3: Smoke-test SourceForge finder without installing to a real destination**

Use a temporary output directory:

```bash
mkdir .tmp-sourceforge-smoke
go run ./cmd/eget install sourceforge:winmerge/stable --asset x64,setup --to .tmp-sourceforge-smoke --quiet
```

Expected:

- The command resolves a SourceForge asset URL.
- If the asset is an installer and prompts, cancel the prompt; prompt behavior is acceptable for GUI installer assets.
- If cancelled, the error should be `installer launch cancelled`, not a SourceForge discovery error.

- [ ] **Step 4: Smoke-test docs example parse path**

Run:

```bash
go test ./internal/config ./internal/app ./internal/install ./internal/source/sourceforge -v
```

Expected: PASS.

- [ ] **Step 5: Final commit if smoke-test fixes were needed**

Only run this if Step 3 or Step 4 required code/doc changes:

```bash
git add .
git commit -m "fix(sourceforge): address smoke test issues"
```

---

## Self-Review

- Spec coverage: direct `sourceforge:<project>` targets, managed config, `source_path`, SourceForge finder, latest checks, installed-store metadata, docs, and no old-asset fallback are covered.
- Placeholder scan: no unfinished markers or unspecified test commands remain.
- Type consistency: `SourcePath` is used consistently in config, install options, app list/update flows, and docs.
- Scope check: SourceForge search and query parity are intentionally excluded as non-goals.
