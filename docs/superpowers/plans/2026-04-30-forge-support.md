# Forge 支持实现计划

> **给 agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 通过 `gitlab:group/project`、`gitea:codeberg.org/owner/repo` 等 provider-prefixed target，支持公开 GitLab、Gitea、Forgejo release asset 的安装、下载、更新检查。

**架构：** 新增 `internal/source/forge` 后端，负责解析 host-aware forge target、调用 GitLab 或 Gitea-compatible release API，并返回候选下载 URL。后续的 asset 筛选、下载、校验、提取、installed store 记录继续复用现有 install runner。

**技术栈：** Go 1.24.2、现有 `net/http` getter 接口风格、GitLab Releases API v4、Gitea/Forgejo Releases API v1、现有 config/installed TOML model、`go test ./...`。

---

## 文件结构

- 新增 `internal/source/forge/target.go`：解析并规范化 `gitlab:`、`gitea:`、`forgejo:` target。
- 新增 `internal/source/forge/target_test.go`：target parser 测试。
- 新增 `internal/source/forge/finder.go`：共享 finder、latest info API、provider dispatch、通用错误。
- 新增 `internal/source/forge/gitlab.go`：GitLab release API URL 构造和 JSON 解析。
- 新增 `internal/source/forge/gitlab_test.go`：GitLab API parser/finder 测试。
- 新增 `internal/source/forge/gitea.go`：Gitea/Forgejo release API URL 构造和 JSON 解析。
- 新增 `internal/source/forge/gitea_test.go`：Gitea/Forgejo API parser/finder 测试。
- 新增 `internal/source/forge/log.go`：verbose logging 和 test helper。
- 修改 `internal/install/options.go`：新增 `TargetForge` 并识别 forge target。
- 修改 `internal/install/service.go`：把 forge target 路由到 `forge.Finder`。
- 修改 `internal/install/defaults.go`：配置 forge HTTP getter factory。
- 修改 `internal/install/service_test.go`、`internal/install/options_test.go`：安装选择测试。
- 修改 `internal/cli/wiring.go`、`internal/cli/service_test.go`：接入 forge getter、latest checker、verbose logger。
- 修改 `internal/app/config.go`、`internal/app/add_test.go`：规范化 managed forge package。
- 修改 `internal/installed/store.go`、`internal/installed/store_test.go`：规范化 installed store key。
- 修改 `internal/app/update.go`、`internal/app/update_test.go`：允许 direct forge update target。
- 修改 `internal/app/install.go`、`internal/app/install_test.go`：记录 forge version。
- 修改 `internal/app/list_test.go`、`internal/app/update_test.go`：混合来源 outdated/update 检查。
- 修改文档：`README.md`、`README.zh-CN.md`、`docs/DOCS.md`、`docs/example.eget.toml`。

## 约束

- 不改变 `owner/repo` 的 GitHub 语义。
- 第一版不自动识别任意 GitLab/Gitea web URL。
- 第一版不实现私有仓库认证。
- 第一版不实现 `query gitlab:...`、`query gitea:...` 或 provider search。
- 不在 `internal/install/runner.go` 增加 provider-specific asset matching。
- 当前 release asset 不匹配时，不回退旧 installed asset。
- 每个任务完成后提交一次。

---

### Task 1: 新增 Forge Target Parser 和 Target Kind 检测

**Files:**
- Create: `internal/source/forge/target.go`
- Create: `internal/source/forge/target_test.go`
- Modify: `internal/install/options.go`
- Modify: `internal/install/options_test.go`

- [x] **Step 1: 编写 forge target parser 失败测试**

新增 `internal/source/forge/target_test.go`：

```go
package forge

import (
	"strings"
	"testing"

	"github.com/gookit/goutil/testutil/assert"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantProvider  Provider
		wantHost      string
		wantNamespace string
		wantProject   string
		wantNorm      string
		wantErr       string
	}{
		{name: "gitlab default host", input: "gitlab:fdroid/fdroidserver", wantProvider: ProviderGitLab, wantHost: "gitlab.com", wantNamespace: "fdroid", wantProject: "fdroidserver", wantNorm: "gitlab:gitlab.com/fdroid/fdroidserver"},
		{name: "gitlab explicit host", input: "gitlab:gitlab.gnome.org/GNOME/gtk", wantProvider: ProviderGitLab, wantHost: "gitlab.gnome.org", wantNamespace: "GNOME", wantProject: "gtk", wantNorm: "gitlab:gitlab.gnome.org/GNOME/gtk"},
		{name: "gitlab nested namespace", input: "gitlab:gitlab.example.com/group/subgroup/project", wantProvider: ProviderGitLab, wantHost: "gitlab.example.com", wantNamespace: "group/subgroup", wantProject: "project", wantNorm: "gitlab:gitlab.example.com/group/subgroup/project"},
		{name: "gitea explicit host", input: "gitea:codeberg.org/forgejo/forgejo", wantProvider: ProviderGitea, wantHost: "codeberg.org", wantNamespace: "forgejo", wantProject: "forgejo", wantNorm: "gitea:codeberg.org/forgejo/forgejo"},
		{name: "forgejo explicit host", input: "forgejo:codeberg.org/forgejo/forgejo", wantProvider: ProviderForgejo, wantHost: "codeberg.org", wantNamespace: "forgejo", wantProject: "forgejo", wantNorm: "forgejo:codeberg.org/forgejo/forgejo"},
		{name: "empty gitlab", input: "gitlab:", wantErr: "gitlab project is required"},
		{name: "gitea missing host", input: "gitea:forgejo/forgejo", wantErr: "gitea host is required"},
		{name: "not forge", input: "sourceforge:winmerge", wantErr: "invalid forge target"},
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
			assert.Eq(t, tt.wantProvider, got.Provider)
			assert.Eq(t, tt.wantHost, got.Host)
			assert.Eq(t, tt.wantNamespace, got.Namespace)
			assert.Eq(t, tt.wantProject, got.Project)
			assert.Eq(t, tt.wantNorm, got.Normalized)
		})
	}
}

func TestIsTarget(t *testing.T) {
	assert.True(t, IsTarget("gitlab:fdroid/fdroidserver"))
	assert.True(t, IsTarget("gitea:codeberg.org/forgejo/forgejo"))
	assert.True(t, IsTarget("forgejo:codeberg.org/forgejo/forgejo"))
	assert.False(t, IsTarget("sourceforge:winmerge"))
	assert.False(t, IsTarget("inhere/markview"))
}
```

- [x] **Step 2: 运行 parser 测试确认失败**

Run:

```bash
go test ./internal/source/forge -run 'TestParseTarget|TestIsTarget' -v
```

Expected: FAIL，原因是 `internal/source/forge` 尚不存在。

- [x] **Step 3: 实现 target parser**

新增 `internal/source/forge/target.go`：

```go
package forge

import (
	"fmt"
	"strings"
)

type Provider string

const (
	ProviderGitLab  Provider = "gitlab"
	ProviderGitea   Provider = "gitea"
	ProviderForgejo Provider = "forgejo"
)

type Target struct {
	Provider   Provider
	Host       string
	Namespace  string
	Project    string
	Normalized string
}

func IsTarget(value string) bool {
	return strings.HasPrefix(value, string(ProviderGitLab)+":") ||
		strings.HasPrefix(value, string(ProviderGitea)+":") ||
		strings.HasPrefix(value, string(ProviderForgejo)+":")
}

func ParseTarget(value string) (Target, error) {
	providerText, rest, ok := strings.Cut(value, ":")
	if !ok || !IsTarget(value) {
		return Target{}, fmt.Errorf("invalid forge target %q", value)
	}
	provider := Provider(providerText)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return Target{}, fmt.Errorf("%s project is required", provider)
	}

	parts := splitPath(rest)
	switch provider {
	case ProviderGitLab:
		return parseGitLab(parts)
	case ProviderGitea, ProviderForgejo:
		return parseGitea(provider, parts)
	default:
		return Target{}, fmt.Errorf("unsupported forge provider %q", provider)
	}
}

func parseGitLab(parts []string) (Target, error) {
	if len(parts) < 2 {
		return Target{}, fmt.Errorf("gitlab project is required")
	}
	host := "gitlab.com"
	namespaceParts := parts[:len(parts)-1]
	project := parts[len(parts)-1]
	if len(parts) >= 3 {
		host = parts[0]
		namespaceParts = parts[1 : len(parts)-1]
	}
	return buildTarget(ProviderGitLab, host, namespaceParts, project)
}

func parseGitea(provider Provider, parts []string) (Target, error) {
	if len(parts) < 3 {
		return Target{}, fmt.Errorf("%s host is required", provider)
	}
	host := parts[0]
	namespaceParts := parts[1 : len(parts)-1]
	project := parts[len(parts)-1]
	return buildTarget(provider, host, namespaceParts, project)
}

func buildTarget(provider Provider, host string, namespaceParts []string, project string) (Target, error) {
	host = strings.TrimSpace(host)
	project = strings.TrimSpace(project)
	if host == "" {
		return Target{}, fmt.Errorf("%s host is required", provider)
	}
	if project == "" {
		return Target{}, fmt.Errorf("%s project is required", provider)
	}
	namespace := strings.Join(namespaceParts, "/")
	if namespace == "" {
		return Target{}, fmt.Errorf("%s namespace is required", provider)
	}
	normalized := string(provider) + ":" + host + "/" + namespace + "/" + project
	return Target{Provider: provider, Host: host, Namespace: namespace, Project: project, Normalized: normalized}, nil
}

func splitPath(value string) []string {
	raw := strings.Split(value, "/")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}
```

- [x] **Step 4: 添加 target kind 测试**

在 `internal/install/options_test.go` 的 `TestDetectTargetKind` cases 中增加：

```go
{name: "gitlab target", target: "gitlab:fdroid/fdroidserver", want: TargetForge},
{name: "gitlab target with host", target: "gitlab:gitlab.gnome.org/GNOME/gtk", want: TargetForge},
{name: "gitea target", target: "gitea:codeberg.org/forgejo/forgejo", want: TargetForge},
{name: "forgejo target", target: "forgejo:codeberg.org/forgejo/forgejo", want: TargetForge},
```

- [x] **Step 5: 实现 target kind 检测**

在 `internal/install/options.go` 导入 forge：

```go
forge "github.com/inherelab/eget/internal/source/forge"
```

新增 target kind：

```go
TargetForge TargetKind = "forge"
```

在 `DetectTargetKind` 中，放在 GitHub URL / direct URL / repo 检测之前：

```go
case forge.IsTarget(target):
	return TargetForge
```

- [x] **Step 6: 运行 target 测试**

Run:

```bash
go test ./internal/source/forge ./internal/install -run 'TestParseTarget|TestIsTarget|TestDetectTargetKind' -v
```

Expected: PASS。

- [x] **Step 7: 提交**

```bash
git add internal/source/forge/target.go internal/source/forge/target_test.go internal/install/options.go internal/install/options_test.go
git commit -m "feat(forge): detect forge targets"
```

---

### Task 2: 实现 GitLab Release Finder

**Files:**
- Create: `internal/source/forge/finder.go`
- Create: `internal/source/forge/gitlab.go`
- Create: `internal/source/forge/gitlab_test.go`
- Create: `internal/source/forge/log.go`

- [x] **Step 1: 编写 GitLab finder 失败测试**

新增 `internal/source/forge/gitlab_test.go`：

```go
package forge

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gookit/goutil/testutil/assert"
)

type fakeGetter struct {
	responses map[string]string
	statuses  map[string]int
	requests  []string
}

func (g *fakeGetter) Get(url string) (*http.Response, error) {
	g.requests = append(g.requests, url)
	status := http.StatusOK
	if g.statuses != nil && g.statuses[url] != 0 {
		status = g.statuses[url]
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(g.responses[url])),
	}, nil
}

func TestGitLabFinderFindsLatestReleaseAssets(t *testing.T) {
	target, err := ParseTarget("gitlab:fdroid/fdroidserver")
	assert.NoErr(t, err)

	url := "https://gitlab.com/api/v4/projects/fdroid%2Ffdroidserver/releases/permalink/latest"
	getter := &fakeGetter{responses: map[string]string{
		url: `{"tag_name":"v2.3.4","assets":{"links":[{"name":"fdroidserver-linux-amd64.tar.gz","direct_asset_url":"https://gitlab.com/fdroid/fdroidserver/-/releases/v2.3.4/downloads/fdroidserver-linux-amd64.tar.gz"},{"name":"fdroidserver.sha256","url":"https://gitlab.com/fdroid/fdroidserver/-/releases/v2.3.4/downloads/fdroidserver.sha256"}]}}`,
	}}

	assets, err := Finder{Target: target, Getter: getter}.Find()

	assert.NoErr(t, err)
	assert.Eq(t, []string{
		"https://gitlab.com/fdroid/fdroidserver/-/releases/v2.3.4/downloads/fdroidserver-linux-amd64.tar.gz",
		"https://gitlab.com/fdroid/fdroidserver/-/releases/v2.3.4/downloads/fdroidserver.sha256",
	}, assets)
	assert.Eq(t, []string{url}, getter.requests)
}

func TestGitLabFinderFindsSpecificTag(t *testing.T) {
	target, err := ParseTarget("gitlab:gitlab.example.com/group/subgroup/project")
	assert.NoErr(t, err)
	url := "https://gitlab.example.com/api/v4/projects/group%2Fsubgroup%2Fproject/releases/v1.0.0"
	getter := &fakeGetter{responses: map[string]string{
		url: `{"tag_name":"v1.0.0","assets":{"links":[{"name":"project.exe","url":"https://gitlab.example.com/group/subgroup/project/-/releases/v1.0.0/downloads/project.exe"}]}}`,
	}}

	assets, err := Finder{Target: target, Tag: "v1.0.0", Getter: getter}.Find()

	assert.NoErr(t, err)
	assert.Eq(t, []string{"https://gitlab.example.com/group/subgroup/project/-/releases/v1.0.0/downloads/project.exe"}, assets)
	assert.Eq(t, []string{url}, getter.requests)
}

func TestGitLabFinderRejectsEmptyAssets(t *testing.T) {
	target, err := ParseTarget("gitlab:fdroid/fdroidserver")
	assert.NoErr(t, err)
	url := "https://gitlab.com/api/v4/projects/fdroid%2Ffdroidserver/releases/permalink/latest"
	getter := &fakeGetter{responses: map[string]string{url: `{"tag_name":"v2.3.4","assets":{"links":[]}}`}}

	_, err = Finder{Target: target, Getter: getter}.Find()

	if err == nil || !strings.Contains(err.Error(), "gitlab release assets not found") {
		t.Fatalf("expected empty assets error, got %v", err)
	}
}

func TestGitLabFinderReportsHTTPStatus(t *testing.T) {
	target, err := ParseTarget("gitlab:fdroid/fdroidserver")
	assert.NoErr(t, err)
	url := "https://gitlab.com/api/v4/projects/fdroid%2Ffdroidserver/releases/permalink/latest"
	getter := &fakeGetter{
		responses: map[string]string{url: `{"message":"404 Not found"}`},
		statuses:  map[string]int{url: http.StatusNotFound},
	}

	_, err = Finder{Target: target, Getter: getter}.Find()

	if err == nil || !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), url) {
		t.Fatalf("expected status and URL error, got %v", err)
	}
}

func TestGitLabLatestVersion(t *testing.T) {
	target, err := ParseTarget("gitlab:fdroid/fdroidserver")
	assert.NoErr(t, err)
	url := "https://gitlab.com/api/v4/projects/fdroid%2Ffdroidserver/releases/permalink/latest"
	getter := &fakeGetter{responses: map[string]string{
		url: `{"tag_name":"v2.3.4","assets":{"links":[{"name":"tool.tar.gz","url":"https://gitlab.com/tool.tar.gz"}]}}`,
	}}

	info, err := LatestVersion(target, getter)

	assert.NoErr(t, err)
	assert.Eq(t, "v2.3.4", info.Tag)
}
```

- [x] **Step 2: 运行 GitLab 测试确认失败**

Run:

```bash
go test ./internal/source/forge -run 'TestGitLab' -v
```

Expected: FAIL，原因是 `Finder` 和 `LatestVersion` 尚未实现。

- [x] **Step 3: 实现 shared finder 和 log helpers**

新增 `internal/source/forge/finder.go`：

```go
package forge

import (
	"fmt"
	"io"
	"net/http"
)

type HTTPGetter interface {
	Get(url string) (*http.Response, error)
}

type Finder struct {
	Target Target
	Tag    string
	Getter HTTPGetter
}

type LatestInfo struct {
	Tag string
}

type releaseInfo struct {
	Tag    string
	Assets []string
}

func (f Finder) Find() ([]string, error) {
	if f.Getter == nil {
		return nil, fmt.Errorf("forge HTTP getter is required")
	}
	release, err := f.release()
	if err != nil {
		return nil, err
	}
	if len(release.Assets) == 0 {
		return nil, fmt.Errorf("%s release assets not found for %s/%s/%s", f.Target.Provider, f.Target.Host, f.Target.Namespace, f.Target.Project)
	}
	return release.Assets, nil
}

func LatestVersion(target Target, getter HTTPGetter) (LatestInfo, error) {
	release, err := Finder{Target: target, Getter: getter}.release()
	if err != nil {
		return LatestInfo{}, err
	}
	if release.Tag == "" {
		return LatestInfo{}, fmt.Errorf("%s latest release tag not found for %s/%s/%s", target.Provider, target.Host, target.Namespace, target.Project)
	}
	return LatestInfo{Tag: release.Tag}, nil
}

func (f Finder) release() (releaseInfo, error) {
	switch f.Target.Provider {
	case ProviderGitLab:
		return f.gitLabRelease()
	case ProviderGitea, ProviderForgejo:
		return releaseInfo{}, fmt.Errorf("%s release finder is not implemented", f.Target.Provider)
	default:
		return releaseInfo{}, fmt.Errorf("unsupported forge provider %q", f.Target.Provider)
	}
}

func (f Finder) getJSON(url string) ([]byte, error) {
	verbosef("forge %s request: %s", f.Target.Provider, url)
	resp, err := f.Getter.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	verbosef("forge %s response: %s", f.Target.Provider, truncateBody(body))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s release request failed: %d %s (URL: %s)", f.Target.Provider, resp.StatusCode, http.StatusText(resp.StatusCode), url)
	}
	return body, nil
}
```

新增 `internal/source/forge/log.go`：

```go
package forge

import (
	"fmt"
	"io"
	"os"
	"strings"
)

var verboseWriter io.Writer = os.Stderr
var verboseEnabled bool

func SetVerbose(enabled bool, writer io.Writer) {
	verboseEnabled = enabled
	if writer == nil {
		verboseWriter = io.Discard
		return
	}
	verboseWriter = writer
}

func verbosef(format string, args ...any) {
	if !verboseEnabled || verboseWriter == nil {
		return
	}
	fmt.Fprintf(verboseWriter, "[verbose] "+format+"\n", args...)
}

func truncateBody(body []byte) string {
	const limit = 240
	text := strings.TrimSpace(string(body))
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "...(truncated)"
}

func VerboseEnabledForTest() bool {
	return verboseEnabled
}
```

- [x] **Step 4: 实现 GitLab API adapter**

新增 `internal/source/forge/gitlab.go`：

```go
package forge

import (
	"encoding/json"
	"net/url"
	"strings"
)

type gitLabRelease struct {
	Tag    string `json:"tag_name"`
	Assets struct {
		Links []struct {
			Name           string `json:"name"`
			URL            string `json:"url"`
			DirectAssetURL string `json:"direct_asset_url"`
		} `json:"links"`
	} `json:"assets"`
}

func (f Finder) gitLabRelease() (releaseInfo, error) {
	apiURL := gitLabReleaseURL(f.Target, f.Tag)
	body, err := f.getJSON(apiURL)
	if err != nil {
		return releaseInfo{}, err
	}
	var release gitLabRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return releaseInfo{}, err
	}
	assets := make([]string, 0, len(release.Assets.Links))
	for _, link := range release.Assets.Links {
		assetURL := strings.TrimSpace(link.DirectAssetURL)
		if assetURL == "" {
			assetURL = strings.TrimSpace(link.URL)
		}
		if assetURL != "" {
			assets = append(assets, assetURL)
		}
	}
	verbosef("forge gitlab assets: %d", len(assets))
	return releaseInfo{Tag: release.Tag, Assets: assets}, nil
}

func gitLabReleaseURL(target Target, tag string) string {
	projectPath := target.Namespace + "/" + target.Project
	encodedProject := url.PathEscape(projectPath)
	base := "https://" + target.Host + "/api/v4/projects/" + encodedProject + "/releases/"
	if strings.TrimSpace(tag) == "" {
		return base + "permalink/latest"
	}
	return base + url.PathEscape(tag)
}
```

- [x] **Step 5: 运行 GitLab 测试**

Run:

```bash
go test ./internal/source/forge -run 'TestGitLab' -v
```

Expected: PASS。

- [x] **Step 6: 提交**

```bash
git add internal/source/forge/finder.go internal/source/forge/gitlab.go internal/source/forge/gitlab_test.go internal/source/forge/log.go
git commit -m "feat(forge): add gitlab release finder"
```

---

### Task 3: 实现 Gitea / Forgejo Release Finder

**Files:**
- Modify: `internal/source/forge/finder.go`
- Create: `internal/source/forge/gitea.go`
- Create: `internal/source/forge/gitea_test.go`

- [x] **Step 1: 编写 Gitea / Forgejo finder 失败测试**

新增 `internal/source/forge/gitea_test.go`：

```go
package forge

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gookit/goutil/testutil/assert"
)

func TestGiteaFinderFindsLatestReleaseAssets(t *testing.T) {
	target, err := ParseTarget("gitea:codeberg.org/forgejo/forgejo")
	assert.NoErr(t, err)
	url := "https://codeberg.org/api/v1/repos/forgejo/forgejo/releases/latest"
	getter := &fakeGetter{responses: map[string]string{
		url: `{"tag_name":"v9.0.0","assets":[{"name":"forgejo-linux-amd64","browser_download_url":"https://codeberg.org/forgejo/forgejo/releases/download/v9.0.0/forgejo-linux-amd64"},{"name":"forgejo.sha256","browser_download_url":"https://codeberg.org/forgejo/forgejo/releases/download/v9.0.0/forgejo.sha256"}]}`,
	}}

	assets, err := Finder{Target: target, Getter: getter}.Find()

	assert.NoErr(t, err)
	assert.Eq(t, []string{
		"https://codeberg.org/forgejo/forgejo/releases/download/v9.0.0/forgejo-linux-amd64",
		"https://codeberg.org/forgejo/forgejo/releases/download/v9.0.0/forgejo.sha256",
	}, assets)
	assert.Eq(t, []string{url}, getter.requests)
}

func TestForgejoFinderUsesGiteaCompatibleAPI(t *testing.T) {
	target, err := ParseTarget("forgejo:codeberg.org/forgejo/forgejo")
	assert.NoErr(t, err)
	url := "https://codeberg.org/api/v1/repos/forgejo/forgejo/releases/tags/v9.0.0"
	getter := &fakeGetter{responses: map[string]string{
		url: `{"tag_name":"v9.0.0","assets":[{"name":"forgejo.exe","browser_download_url":"https://codeberg.org/forgejo/forgejo/releases/download/v9.0.0/forgejo.exe"}]}`,
	}}

	assets, err := Finder{Target: target, Tag: "v9.0.0", Getter: getter}.Find()

	assert.NoErr(t, err)
	assert.Eq(t, []string{"https://codeberg.org/forgejo/forgejo/releases/download/v9.0.0/forgejo.exe"}, assets)
	assert.Eq(t, []string{url}, getter.requests)
}

func TestGiteaFinderRejectsEmptyAssets(t *testing.T) {
	target, err := ParseTarget("gitea:codeberg.org/forgejo/forgejo")
	assert.NoErr(t, err)
	url := "https://codeberg.org/api/v1/repos/forgejo/forgejo/releases/latest"
	getter := &fakeGetter{responses: map[string]string{url: `{"tag_name":"v9.0.0","assets":[]}`}}

	_, err = Finder{Target: target, Getter: getter}.Find()

	if err == nil || !strings.Contains(err.Error(), "gitea release assets not found") {
		t.Fatalf("expected empty assets error, got %v", err)
	}
}

func TestGiteaFinderReportsHTTPStatus(t *testing.T) {
	target, err := ParseTarget("gitea:codeberg.org/forgejo/forgejo")
	assert.NoErr(t, err)
	url := "https://codeberg.org/api/v1/repos/forgejo/forgejo/releases/latest"
	getter := &fakeGetter{
		responses: map[string]string{url: `{"message":"not found"}`},
		statuses:  map[string]int{url: http.StatusNotFound},
	}

	_, err = Finder{Target: target, Getter: getter}.Find()

	if err == nil || !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), url) {
		t.Fatalf("expected status and URL error, got %v", err)
	}
}

func TestGiteaLatestVersion(t *testing.T) {
	target, err := ParseTarget("gitea:codeberg.org/forgejo/forgejo")
	assert.NoErr(t, err)
	url := "https://codeberg.org/api/v1/repos/forgejo/forgejo/releases/latest"
	getter := &fakeGetter{responses: map[string]string{
		url: `{"tag_name":"v9.0.0","assets":[{"name":"forgejo","browser_download_url":"https://codeberg.org/forgejo"}]}`,
	}}

	info, err := LatestVersion(target, getter)

	assert.NoErr(t, err)
	assert.Eq(t, "v9.0.0", info.Tag)
}
```

- [x] **Step 2: 运行 Gitea 测试确认失败**

Run:

```bash
go test ./internal/source/forge -run 'TestGitea|TestForgejo' -v
```

Expected: FAIL，原因是 `giteaRelease` 尚未实现。

- [x] **Step 3: 实现 Gitea API adapter**

先在 `internal/source/forge/finder.go` 中把 `release()` 的 Gitea/Forgejo 分支改为调用 adapter：

```go
func (f Finder) release() (releaseInfo, error) {
	switch f.Target.Provider {
	case ProviderGitLab:
		return f.gitLabRelease()
	case ProviderGitea, ProviderForgejo:
		return f.giteaRelease()
	default:
		return releaseInfo{}, fmt.Errorf("unsupported forge provider %q", f.Target.Provider)
	}
}
```

新增 `internal/source/forge/gitea.go`：

```go
package forge

import (
	"encoding/json"
	"net/url"
	"strings"
)

type giteaRelease struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func (f Finder) giteaRelease() (releaseInfo, error) {
	apiURL := giteaReleaseURL(f.Target, f.Tag)
	body, err := f.getJSON(apiURL)
	if err != nil {
		return releaseInfo{}, err
	}
	var release giteaRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return releaseInfo{}, err
	}
	assets := make([]string, 0, len(release.Assets))
	for _, asset := range release.Assets {
		assetURL := strings.TrimSpace(asset.BrowserDownloadURL)
		if assetURL != "" {
			assets = append(assets, assetURL)
		}
	}
	verbosef("forge %s assets: %d", f.Target.Provider, len(assets))
	return releaseInfo{Tag: release.Tag, Assets: assets}, nil
}

func giteaReleaseURL(target Target, tag string) string {
	repoPath := strings.Trim(target.Namespace+"/"+target.Project, "/")
	base := "https://" + target.Host + "/api/v1/repos/" + repoPath + "/releases/"
	if strings.TrimSpace(tag) == "" {
		return base + "latest"
	}
	return base + "tags/" + url.PathEscape(tag)
}
```

- [x] **Step 4: 运行 forge 包测试**

Run:

```bash
go test ./internal/source/forge -v
```

Expected: PASS。

- [x] **Step 5: 提交**

```bash
git add internal/source/forge/gitea.go internal/source/forge/gitea_test.go internal/source/forge/finder.go
git commit -m "feat(forge): add gitea release finder"
```

---

### Task 4: 接入 Install Service 和 CLI

**Files:**
- Modify: `internal/install/service.go`
- Modify: `internal/install/defaults.go`
- Modify: `internal/install/service_test.go`
- Modify: `internal/cli/wiring.go`
- Modify: `internal/cli/service_test.go`

- [x] **Step 1: 编写 install service 选择测试**

在 `internal/install/service_test.go` 导入 forge：

```go
forge "github.com/inherelab/eget/internal/source/forge"
```

在 `TestSelectFinder` 内新增：

```go
t.Run("forge gitlab target", func(t *testing.T) {
	opts := &Options{Tag: "v1.2.3", ProxyURL: "http://127.0.0.1:7890"}
	svc.ForgeGetterFactory = func(opts Options) forge.HTTPGetter {
		return fakeHTTPGetterFunc(func(url string) (*http.Response, error) {
			if opts.ProxyURL != "http://127.0.0.1:7890" {
				t.Fatalf("expected proxy url to propagate to forge getter, got %q", opts.ProxyURL)
			}
			return nil, nil
		})
	}

	finder, tool, err := svc.SelectFinder("gitlab:fdroid/fdroidserver", opts)
	if err != nil {
		t.Fatalf("SelectFinder(gitlab): %v", err)
	}
	if tool != "fdroidserver" {
		t.Fatalf("tool = %q, want fdroidserver", tool)
	}
	got, ok := finder.(forge.Finder)
	if !ok {
		t.Fatalf("finder type = %T, want forge.Finder", finder)
	}
	if got.Target.Normalized != "gitlab:gitlab.com/fdroid/fdroidserver" || got.Tag != "v1.2.3" || got.Getter == nil {
		t.Fatalf("unexpected forge finder: %+v", got)
	}
})

t.Run("forge gitea target", func(t *testing.T) {
	svc.ForgeGetterFactory = func(opts Options) forge.HTTPGetter {
		return fakeHTTPGetterFunc(func(url string) (*http.Response, error) { return nil, nil })
	}

	finder, tool, err := svc.SelectFinder("gitea:codeberg.org/forgejo/forgejo", &Options{})
	if err != nil {
		t.Fatalf("SelectFinder(gitea): %v", err)
	}
	if tool != "forgejo" {
		t.Fatalf("tool = %q, want forgejo", tool)
	}
	got, ok := finder.(forge.Finder)
	if !ok || got.Target.Provider != forge.ProviderGitea {
		t.Fatalf("finder type = %T value=%+v, want gitea forge.Finder", finder, got)
	}
})
```

- [x] **Step 2: 编写 verbose wiring 测试**

在 `internal/cli/service_test.go` 导入 forge：

```go
forge "github.com/inherelab/eget/internal/source/forge"
```

扩展 `TestConfigureVerboseUpdatesVerboseLoggers`：

```go
if !forge.VerboseEnabledForTest() {
	t.Fatalf("expected forge verbose to be enabled")
}
```

- [x] **Step 3: 运行测试确认失败**

Run:

```bash
go test ./internal/install ./internal/cli -run 'TestSelectFinder|TestConfigureVerboseUpdatesVerboseLoggers' -v
```

Expected: FAIL，原因是 forge factory 和 CLI verbose wiring 尚未接入。

- [x] **Step 4: 在 install service 接入 forge finder**

在 `internal/install/service.go` 导入 forge：

```go
forge "github.com/inherelab/eget/internal/source/forge"
```

给 `Service` 增加：

```go
ForgeGetterFactory func(opts Options) forge.HTTPGetter
```

在 `SelectFinder` 中新增 case：

```go
case TargetForge:
	forgeTarget, err := forge.ParseTarget(target)
	if err != nil {
		return nil, "", err
	}
	if s.ForgeGetterFactory == nil {
		return nil, "", fmt.Errorf("forge getter factory is required")
	}
	return forge.Finder{
		Target: forgeTarget,
		Tag:    opts.Tag,
		Getter: s.ForgeGetterFactory(*opts),
	}, forgeTarget.Project, nil
```

- [x] **Step 5: 接入 default service 和 CLI**

在 `internal/install/defaults.go` 导入 forge，并添加：

```go
ForgeGetterFactory: func(opts Options) forge.HTTPGetter {
	return NewHTTPGetter(opts)
},
```

在 `internal/cli/wiring.go` 导入 forge，并添加：

```go
installService.ForgeGetterFactory = func(opts install.Options) forge.HTTPGetter {
	return client.NewGitHubClient(install.ClientOptions(opts))
}
```

在 `configureVerbose` 中添加：

```go
forge.SetVerbose(verbose, stderr)
```

- [x] **Step 6: 运行 wiring 测试**

Run:

```bash
go test ./internal/install ./internal/cli -run 'TestSelectFinder|TestConfigureVerboseUpdatesVerboseLoggers|TestNewDefaultServiceWiring' -v
```

Expected: PASS。

- [x] **Step 7: 提交**

```bash
git add internal/install/service.go internal/install/defaults.go internal/install/service_test.go internal/cli/wiring.go internal/cli/service_test.go
git commit -m "feat(forge): wire install finder"
```

---

### Task 5: 规范化 Config 和 Installed Store 中的 Forge Target

**Files:**
- Modify: `internal/app/config.go`
- Modify: `internal/app/add_test.go`
- Modify: `internal/installed/store.go`
- Modify: `internal/installed/store_test.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/update_test.go`

- [x] **Step 1: 编写 add package normalization 测试**

在 `internal/app/add_test.go` 新增：

```go
func TestAddPackageNormalizesForgeTargets(t *testing.T) {
	tests := []struct {
		name     string
		repo     string
		wantName string
		wantRepo string
	}{
		{name: "gitlab default host", repo: "gitlab:fdroid/fdroidserver", wantName: "fdroidserver", wantRepo: "gitlab:gitlab.com/fdroid/fdroidserver"},
		{name: "gitea explicit host", repo: "gitea:codeberg.org/forgejo/forgejo", wantName: "forgejo", wantRepo: "gitea:codeberg.org/forgejo/forgejo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			configPath := filepath.Join(tmp, "eget.toml")
			svc := ConfigService{
				ConfigPath: configPath,
				Load: func() (*cfgpkg.File, error) {
					return cfgpkg.NewFile(), nil
				},
				Save: cfgpkg.Save,
			}

			if err := svc.AddPackage(tt.repo, "", install.Options{}); err != nil {
				t.Fatalf("add forge package: %v", err)
			}

			cfg, err := cfgpkg.LoadFile(configPath)
			if err != nil {
				t.Fatalf("load config: %v", err)
			}
			pkg, ok := cfg.Packages[tt.wantName]
			if !ok {
				t.Fatalf("expected packages.%s, got %#v", tt.wantName, cfg.Packages)
			}
			if pkg.Repo == nil || *pkg.Repo != tt.wantRepo {
				t.Fatalf("expected normalized repo %q, got %#v", tt.wantRepo, pkg.Repo)
			}
		})
	}
}
```

- [x] **Step 2: 编写 installed normalization 测试**

在 `internal/installed/store_test.go` 新增：

```go
func TestNormalizeRepoNameForge(t *testing.T) {
	assert.Eq(t, "gitlab:gitlab.com/fdroid/fdroidserver", NormalizeRepoName("gitlab:fdroid/fdroidserver"))
	assert.Eq(t, "gitlab:gitlab.gnome.org/GNOME/gtk", NormalizeRepoName("gitlab:gitlab.gnome.org/GNOME/gtk"))
	assert.Eq(t, "gitea:codeberg.org/forgejo/forgejo", NormalizeRepoName("gitea:codeberg.org/forgejo/forgejo"))
	assert.Eq(t, "forgejo:codeberg.org/forgejo/forgejo", NormalizeRepoName("forgejo:codeberg.org/forgejo/forgejo"))
}
```

- [x] **Step 3: 编写 direct update target 测试**

在 `internal/app/update_test.go` 新增：

```go
func TestUpdatePackageAllowsDirectForgeTargets(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
	}

	_, err := svc.UpdatePackage("gitlab:fdroid/fdroidserver", install.Options{})
	assert.NoErr(t, err)
	_, err = svc.UpdatePackage("gitea:codeberg.org/forgejo/forgejo", install.Options{})
	assert.NoErr(t, err)
	assert.Eq(t, []string{"gitlab:fdroid/fdroidserver", "gitea:codeberg.org/forgejo/forgejo"}, installer.targets)
}
```

- [x] **Step 4: 运行测试确认失败**

Run:

```bash
go test ./internal/app ./internal/installed -run 'TestAddPackageNormalizesForgeTargets|TestNormalizeRepoNameForge|TestUpdatePackageAllowsDirectForgeTargets' -v
```

Expected: FAIL，原因是 forge normalization 和 direct update routing 尚未实现。

- [x] **Step 5: 实现 app config normalization**

在 `internal/app/config.go` 导入 forge：

```go
forge "github.com/inherelab/eget/internal/source/forge"
```

在 `AddPackage` 中 SourceForge normalization 后、GitHub basename fallback 前增加：

```go
if forgeTarget, forgeErr := forge.ParseTarget(repo); forgeErr == nil {
	repo = forgeTarget.Normalized
	if name == "" {
		name = forgeTarget.Project
	}
}
```

- [x] **Step 6: 实现 installed normalization**

在 `internal/installed/store.go` 导入 forge：

```go
forge "github.com/inherelab/eget/internal/source/forge"
```

在 `NormalizeRepoName` 顶部增加：

```go
if forgeTarget, err := forge.ParseTarget(target); err == nil {
	return forgeTarget.Normalized
}
```

- [x] **Step 7: 允许 direct forge update target**

在 `internal/app/update.go` 导入 forge：

```go
forge "github.com/inherelab/eget/internal/source/forge"
```

更新 direct target 条件：

```go
if strings.Contains(nameOrRepo, "/") || sourceforge.IsTarget(nameOrRepo) || forge.IsTarget(nameOrRepo) {
	return s.Install.InstallTarget(nameOrRepo, cli)
}
```

- [x] **Step 8: 运行 normalization 测试**

Run:

```bash
go test ./internal/app ./internal/installed -run 'TestAddPackageNormalizesForgeTargets|TestNormalizeRepoNameForge|TestUpdatePackageAllowsDirectForgeTargets' -v
```

Expected: PASS。

- [x] **Step 9: 提交**

```bash
git add internal/app/config.go internal/app/add_test.go internal/installed/store.go internal/installed/store_test.go internal/app/update.go internal/app/update_test.go
git commit -m "feat(forge): normalize managed targets"
```

---

### Task 6: 支持 Forge Latest Checks 和安装元数据

**Files:**
- Modify: `internal/cli/wiring.go`
- Modify: `internal/app/install.go`
- Modify: `internal/app/install_test.go`
- Modify: `internal/app/list_test.go`
- Modify: `internal/app/update_test.go`

- [x] **Step 1: 编写安装元数据测试**

在 `internal/app/install_test.go` 新增：

```go
func TestInstallTargetRecordsForgeVersionFromReleaseInfo(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://gitlab.com/fdroid/fdroidserver/-/releases/v2.3.4/downloads/fdroidserver-linux-amd64.tar.gz",
			Tool:           "fdroidserver",
			ExtractedFiles: []string{"./fdroidserver"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{
		Runner: runner,
		Store:  store,
		ReleaseInfo: func(repo, url string) (string, time.Time, error) {
			if repo != "gitlab:gitlab.com/fdroid/fdroidserver" {
				t.Fatalf("unexpected repo %q", repo)
			}
			return "v2.3.4", now.Add(-time.Hour), nil
		},
	}

	_, err := svc.InstallTarget("gitlab:fdroid/fdroidserver", install.Options{})
	if err != nil {
		t.Fatalf("install forge target: %v", err)
	}

	assert.Eq(t, "gitlab:gitlab.com/fdroid/fdroidserver", store.entry.Repo)
	assert.Eq(t, "v2.3.4", store.entry.Tag)
	assert.Eq(t, "v2.3.4", store.entry.Version)
}
```

- [x] **Step 2: 编写 list outdated forge 测试**

在 `internal/app/list_test.go` 新增：

```go
func TestListOutdatedPackagesChecksForgeRepo(t *testing.T) {
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["forgejo"] = cfgpkg.Section{Repo: util.StringPtr("gitea:codeberg.org/forgejo/forgejo")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"gitea:codeberg.org/forgejo/forgejo": {Repo: "gitea:codeberg.org/forgejo/forgejo", Tag: "v8.0.0"},
			}}, nil
		},
		LatestTag: func(repo, sourcePath string) (string, error) {
			if repo != "gitea:codeberg.org/forgejo/forgejo" || sourcePath != "" {
				t.Fatalf("unexpected latest check repo=%q sourcePath=%q", repo, sourcePath)
			}
			return "v9.0.0", nil
		},
	}

	items, failures, checked, err := svc.ListOutdatedPackages()

	assert.NoErr(t, err)
	assert.Eq(t, 1, checked)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 1, len(items))
	assert.Eq(t, "v9.0.0", items[0].LatestTag)
}
```

- [x] **Step 3: 编写 update candidate forge 测试**

在 `internal/app/update_test.go` 新增：

```go
func TestListUpdateCandidatesChecksForgeRepo(t *testing.T) {
	svc := UpdateService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["fdroidserver"] = cfgpkg.Section{Repo: util.StringPtr("gitlab:gitlab.com/fdroid/fdroidserver")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"gitlab:gitlab.com/fdroid/fdroidserver": {Repo: "gitlab:gitlab.com/fdroid/fdroidserver", Tag: "v2.3.3"},
			}}, nil
		},
		LatestTag: func(repo, sourcePath string) (string, error) {
			if repo != "gitlab:gitlab.com/fdroid/fdroidserver" || sourcePath != "" {
				t.Fatalf("unexpected latest check repo=%q sourcePath=%q", repo, sourcePath)
			}
			return "v2.3.4", nil
		},
	}

	items, failures, checked, err := svc.ListUpdateCandidates()

	assert.NoErr(t, err)
	assert.Eq(t, 1, checked)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 1, len(items))
	assert.Eq(t, "v2.3.4", items[0].LatestTag)
}
```

- [x] **Step 4: 运行 app 测试确认当前缺口**

Run:

```bash
go test ./internal/app -run 'TestInstallTargetRecordsForgeVersionFromReleaseInfo|TestListOutdatedPackagesChecksForgeRepo|TestListUpdateCandidatesChecksForgeRepo' -v
```

Expected: metadata test 在 forge repo `Version` 未写入前失败；list/update 测试可能已经通过，保留作为回归覆盖。

- [x] **Step 5: 在 installed entry 记录 forge version**

在 `internal/app/install.go` 导入 forge：

```go
forge "github.com/inherelab/eget/internal/source/forge"
```

把 SourceForge-only 版本判断扩展为：

```go
isSourceForge := sourcesf.IsTarget(repo)
isForge := forge.IsTarget(repo)
if tag == "" && isSourceForge {
	tag = sourcesf.VersionFromText(result.URL)
}
```

更新 entry version：

```go
Version: sourceVersion(tag, isSourceForge || isForge),
```

保留 helper：

```go
func sourceVersion(tag string, sourceBacked bool) string {
	if sourceBacked {
		return tag
	}
	return ""
}
```

- [x] **Step 6: 在 CLI 接入 forge latest checker**

在 `internal/cli/wiring.go` 的 `latestTag` closure 中，GitHub fallback 前增加：

```go
if forgeTarget, err := forge.ParseTarget(repo); err == nil {
	info, err := forge.LatestVersion(forgeTarget, install.NewHTTPGetter(defaultOpts))
	if err != nil {
		return "", err
	}
	return info.Tag, nil
}
```

扩展 `appService.ReleaseInfo`：

```go
ReleaseInfo: func(repo, url string) (string, time.Time, error) {
	if forgeTarget, err := forge.ParseTarget(repo); err == nil {
		info, err := forge.LatestVersion(forgeTarget, install.NewHTTPGetter(defaultOpts))
		return info.Tag, time.Time{}, err
	}
	return githubClient.LatestReleaseInfo(repo)
},
```

保持 SourceForge 行为不变；SourceForge install metadata 仍可从 URL 解析。

- [x] **Step 7: 运行聚焦测试**

Run:

```bash
go test ./internal/app ./internal/cli -run 'Forge|Outdated|UpdateCandidates|NewCLIService' -v
```

Expected: PASS。

- [x] **Step 8: 提交**

```bash
git add internal/app/install.go internal/app/install_test.go internal/app/list_test.go internal/app/update_test.go internal/cli/wiring.go
git commit -m "feat(forge): support latest checks"
```

---

### Task 7: 更新文档和示例配置

**Files:**
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `docs/DOCS.md`
- Modify: `docs/example.eget.toml`

- [x] **Step 1: 更新英文 README 示例**

在 `README.md` 增加安装示例：

```markdown
# Install from GitLab releases
eget install gitlab:fdroid/fdroidserver
eget install gitlab:gitlab.gnome.org/GNOME/gtk

# Install from Gitea/Forgejo-compatible releases
eget install gitea:codeberg.org/forgejo/forgejo --asset linux,amd64
```

增加 supported target bullet：

```markdown
- GitLab target, for example `gitlab:fdroid/fdroidserver` or `gitlab:gitlab.gnome.org/GNOME/gtk`
- Gitea/Forgejo target, for example `gitea:codeberg.org/forgejo/forgejo`
```

增加配置示例：

```toml
[packages.forgejo]
repo = "gitea:codeberg.org/forgejo/forgejo"
system = "linux/amd64"
asset_filters = ["linux", "amd64"]
```

- [x] **Step 2: 更新中文 README 示例**

在 `README.zh-CN.md` 增加对应中文示例：

```markdown
# 从 GitLab releases 安装
eget install gitlab:fdroid/fdroidserver
eget install gitlab:gitlab.gnome.org/GNOME/gtk

# 从 Gitea/Forgejo-compatible releases 安装
eget install gitea:codeberg.org/forgejo/forgejo --asset linux,amd64
```

同时增加 target bullet 和 TOML 示例，说明 GitLab/Gitea 第一版只支持 install/download/update release assets。

- [x] **Step 3: 更新架构文档**

在 `docs/DOCS.md` 增加 runtime layout bullet：

```markdown
- `internal/source/forge`: GitLab/Gitea/Forgejo release asset 发现与 latest-version 检查
```

增加 install target 支持：

```markdown
- Forge target，例如 `gitlab:fdroid/fdroidserver`、`gitea:codeberg.org/forgejo/forgejo`
```

增加 flow section：

```markdown
## Forge Flow

`gitlab:`、`gitea:`、`forgejo:` 目标由 `internal/source/forge` 解析并调用对应公开 release API。
Forge 后端只返回候选下载 URL；`system`、`asset_filters`、`file`、下载、校验、提取和 installed store 记录继续复用普通安装链路。
第一版不支持私有仓库认证、query/search parity 或从任意网页 URL 自动识别 provider。
```

- [x] **Step 4: 更新 example config**

在 `docs/example.eget.toml` 增加：

```toml
[packages."forgejo"]
name = "forgejo"
repo = "gitea:codeberg.org/forgejo/forgejo"
system = "linux/amd64"
asset_filters = ["linux", "amd64"]
```

- [x] **Step 5: 运行文档邻近测试**

Run:

```bash
go test ./internal/config ./internal/app ./internal/install ./internal/source/forge -v
```

Expected: PASS。

- [x] **Step 6: 提交 docs**

```bash
git add README.md README.zh-CN.md docs/DOCS.md docs/example.eget.toml
git commit -m "docs: document forge packages"
```

---

### Task 8: 全量验证和 Smoke Tests

**Files:**
- smoke test 暴露问题时才修改相关代码或文档。
- 完成前更新本 checklist。

- [ ] **Step 1: 运行全量测试**

Run:

```bash
go test ./...
```

Expected: all packages PASS。

- [ ] **Step 2: 构建二进制**

Run:

```bash
go build ./cmd/eget
```

Expected: 命令成功退出，Windows 下生成 `eget.exe`。

- [ ] **Step 3: Smoke-test Gitea/Forgejo release discovery**

使用临时输出目录：

```bash
New-Item -ItemType Directory -Force .tmp-forge-smoke | Out-Null
go run ./cmd/eget download --asset linux,amd64 --to .tmp-forge-smoke gitea:codeberg.org/forgejo/forgejo
```

Expected:

- 命令能解析 Gitea/Forgejo release assets。
- 如果多个 Linux amd64 assets 匹配，命令可能进入 prompt；此时收窄 `--asset` 过滤条件，并把可工作的过滤条件写入文档。
- 不应因 target parsing、API response parsing 或 getter 缺失失败。

- [ ] **Step 4: Smoke-test GitLab release discovery**

先确认一个稳定公开 GitLab 项目，且 latest release 包含可下载 assets。优先选择 asset 文件名包含 OS/arch 的项目。

使用确认后的项目运行：

```bash
go run ./cmd/eget download --asset linux,amd64 --to .tmp-forge-smoke gitlab:<known-public-project>
```

Expected:

- 命令能解析 GitLab release assets。
- 不应因 project path encoding、API response parsing 或 getter 缺失失败。
- 如果候选公开项目没有合适 latest release assets，记录为 smoke-target 问题并更换项目。

- [ ] **Step 5: Smoke-test managed config parse path**

Run:

```bash
go test ./internal/config ./internal/app ./internal/install ./internal/source/forge -v
```

Expected: PASS。

- [ ] **Step 6: 如 smoke 暴露问题则提交修复**

只有 smoke test 需要代码或文档修复时才运行：

```bash
git add .
git commit -m "fix(forge): address smoke test issues"
```

---

## Self-Review

- Spec coverage: target 语法、规范化、GitLab API、Gitea/Forgejo API、install routing、latest checks、installed metadata、文档和非目标都已有任务覆盖。
- Scope check: private auth、query/search parity、任意 URL 自动识别、package registry、GitLab job artifacts、provider search 均保持在非目标范围。
- Type consistency: `forge.Target`、`forge.Finder`、`forge.LatestVersion`、`TargetForge`、`ForgeGetterFactory` 都在后续任务使用前定义。
- Test coverage: 每个行为都有先写失败测试的步骤；最终验证包含 `go test ./...`、build 和公开 smoke tests。
