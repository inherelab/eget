# GitHub Search Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 `eget search` 命令，支持 GitHub 仓库搜索、保留原生搜索语法、表格输出与 JSON 输出。

**Architecture:** 新增独立的 `search` 命令、app service 和 GitHub search client，不复用 `query` 的 repo-only 输入模型。CLI 层负责把 `keyword + extras` 拼成 GitHub 搜索串；app 层负责参数校验与默认值；client 层负责调用 `/search/repositories` 并解析结果。

**Tech Stack:** Go 1.24、`gookit/cflag/capp`、现有 CLI table 输出、GitHub REST API `/search/repositories`

---

## File Map

- Create: `internal/cli/search_cmd.go`
- Create: `internal/app/search.go`
- Create: `internal/app/search_test.go`
- Create: `internal/cli/search_client.go`
- Create: `internal/cli/search_client_test.go`
- Modify: `internal/cli/app.go`
- Modify: `internal/cli/service.go`
- Modify: `internal/cli/app_test.go`
- Modify: `internal/cli/service_test.go`

### Responsibility Notes

- `internal/cli/search_cmd.go`: 命令注册、flag 解析、参数快照重置
- `internal/app/search.go`: SearchOptions、SearchResult、SearchRepo、SearchService
- `internal/app/search_test.go`: app 层默认值、校验、参数透传测试
- `internal/cli/search_client.go`: GitHub API 请求和 JSON 解析
- `internal/cli/search_client_test.go`: URL 构造、非 200 处理、响应解析测试
- `internal/cli/service.go`: wiring、命令分发、输出格式
- `internal/cli/app_test.go`: `search` 命令参数解析测试
- `internal/cli/service_test.go`: CLI handle + 输出测试

## Task 1: 定义 app 层搜索模型和失败测试

**Files:**
- Create: `internal/app/search.go`
- Create: `internal/app/search_test.go`

- [ ] **Step 1: 写 app 层失败测试**

```go
func TestSearchServiceAppliesDefaultsAndCombinesQuery(t *testing.T) {
	client := &fakeSearchClient{}
	svc := SearchService{Client: client}

	result, err := svc.Search(SearchOptions{Keyword: "ripgrep", Extras: []string{"language:rust"}})
	if err != nil {
		t.Fatalf("Search(): %v", err)
	}
	if client.lastQuery != "ripgrep language:rust" {
		t.Fatalf("expected combined query, got %q", client.lastQuery)
	}
	if client.lastLimit != 10 {
		t.Fatalf("expected default limit 10, got %d", client.lastLimit)
	}
	if result.Query != "ripgrep language:rust" {
		t.Fatalf("expected result query, got %q", result.Query)
	}
}

func TestSearchServiceRejectsEmptyKeyword(t *testing.T) {
	svc := SearchService{Client: &fakeSearchClient{}}

	_, err := svc.Search(SearchOptions{})
	if err == nil {
		t.Fatal("expected empty keyword to fail")
	}
}

func TestSearchServiceRejectsInvalidSortAndOrder(t *testing.T) {
	svc := SearchService{Client: &fakeSearchClient{}}

	if _, err := svc.Search(SearchOptions{Keyword: "ripgrep", Sort: "forks"}); err == nil {
		t.Fatal("expected invalid sort to fail")
	}
	if _, err := svc.Search(SearchOptions{Keyword: "ripgrep", Order: "sideways"}); err == nil {
		t.Fatal("expected invalid order to fail")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/app -run SearchService`
Expected: FAIL，提示 `SearchService` / `SearchOptions` / `fakeSearchClient` 未定义

- [ ] **Step 3: 写最小 app 实现**

```go
type SearchRepo struct {
	FullName    string    `json:"full_name"`
	Description string    `json:"description,omitempty"`
	HTMLURL     string    `json:"html_url,omitempty"`
	Homepage    string    `json:"homepage,omitempty"`
	Language    string    `json:"language,omitempty"`
	Stars       int       `json:"stargazers_count,omitempty"`
	Forks       int       `json:"forks_count,omitempty"`
	OpenIssues  int       `json:"open_issues_count,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
	Archived    bool      `json:"archived,omitempty"`
	Private     bool      `json:"private,omitempty"`
}

type SearchOptions struct {
	Keyword string
	Extras  []string
	Limit   int
	Sort    string
	Order   string
	JSON    bool
}

type SearchResult struct {
	Query      string       `json:"query"`
	TotalCount int          `json:"total_count"`
	Items      []SearchRepo `json:"items"`
}

type SearchClient interface {
	SearchRepositories(query string, limit int, sort, order string) (SearchResult, error)
}

type SearchService struct {
	Client SearchClient
}

func (s SearchService) Search(opts SearchOptions) (SearchResult, error) {
	if s.Client == nil {
		return SearchResult{}, fmt.Errorf("search client is required")
	}
	keyword := strings.TrimSpace(opts.Keyword)
	if keyword == "" {
		return SearchResult{}, fmt.Errorf("search keyword is required")
	}
	query := keyword
	if len(opts.Extras) > 0 {
		query += " " + strings.Join(opts.Extras, " ")
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	if opts.Sort != "" && opts.Sort != "stars" && opts.Sort != "updated" {
		return SearchResult{}, fmt.Errorf("invalid search sort %q", opts.Sort)
	}
	if opts.Order != "" && opts.Order != "desc" && opts.Order != "asc" {
		return SearchResult{}, fmt.Errorf("invalid search order %q", opts.Order)
	}
	result, err := s.Client.SearchRepositories(query, limit, opts.Sort, opts.Order)
	if err != nil {
		return SearchResult{}, err
	}
	result.Query = query
	return result, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/app -run SearchService`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/app/search.go internal/app/search_test.go
git commit -m "feat: add app search service"
```

## Task 2: 实现 GitHub search client

**Files:**
- Create: `internal/cli/search_client.go`
- Create: `internal/cli/search_client_test.go`

- [ ] **Step 1: 写 client 失败测试**

```go
func TestGitHubSearchClientBuildsSearchURL(t *testing.T) {
	client := newGitHubSearchClient(install.Options{})
	var requested string
	client.get = func(rawURL string, opts install.Options) (*http.Response, error) {
		requested = rawURL
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{"total_count":1,"items":[]}`)),
		}, nil
	}

	_, err := client.SearchRepositories("ripgrep language:rust", 5, "stars", "desc")
	if err != nil {
		t.Fatalf("SearchRepositories(): %v", err)
	}
	if !strings.Contains(requested, "q=ripgrep+language%3Arust") {
		t.Fatalf("expected encoded query, got %q", requested)
	}
	if !strings.Contains(requested, "per_page=5") {
		t.Fatalf("expected per_page=5, got %q", requested)
	}
}

func TestGitHubSearchClientParsesResponse(t *testing.T) {
	client := newGitHubSearchClient(install.Options{})
	client.get = func(rawURL string, opts install.Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{"total_count":1,"items":[{"full_name":"BurntSushi/ripgrep","language":"Rust","stargazers_count":1}]}`)),
		}, nil
	}

	result, err := client.SearchRepositories("ripgrep", 10, "", "")
	if err != nil {
		t.Fatalf("SearchRepositories(): %v", err)
	}
	if result.TotalCount != 1 || len(result.Items) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestGitHubSearchClientHandlesNon200(t *testing.T) {
	client := newGitHubSearchClient(install.Options{})
	client.get = func(rawURL string, opts install.Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Status: "403 Forbidden",
			Body: io.NopCloser(strings.NewReader("rate limited")),
		}, nil
	}

	_, err := client.SearchRepositories("ripgrep", 10, "", "")
	if err == nil {
		t.Fatal("expected non-200 to fail")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/cli -run GitHubSearchClient`
Expected: FAIL，提示 `newGitHubSearchClient` 未定义

- [ ] **Step 3: 写最小 client 实现**

```go
type gitHubSearchClient struct {
	opts install.Options
	get  func(url string, opts install.Options) (*http.Response, error)
}

func newGitHubSearchClient(opts install.Options) *gitHubSearchClient {
	return &gitHubSearchClient{opts: opts, get: githubAPIGetWithOptions}
}

func (c *gitHubSearchClient) SearchRepositories(query string, limit int, sort, order string) (app.SearchResult, error) {
	values := url.Values{}
	values.Set("q", query)
	values.Set("per_page", strconv.Itoa(limit))
	if sort != "" {
		values.Set("sort", sort)
	}
	if order != "" {
		values.Set("order", order)
	}

	resp, err := c.get("https://api.github.com/search/repositories?"+values.Encode(), c.opts)
	if err != nil {
		return app.SearchResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return app.SearchResult{}, fmt.Errorf("search failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload struct {
		TotalCount int              `json:"total_count"`
		Items      []app.SearchRepo `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return app.SearchResult{}, err
	}
	return app.SearchResult{TotalCount: payload.TotalCount, Items: payload.Items}, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/cli -run GitHubSearchClient`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/cli/search_client.go internal/cli/search_client_test.go
git commit -m "feat: add github search client"
```

## Task 3: 注册 search 命令并解析参数

**Files:**
- Create: `internal/cli/search_cmd.go`
- Modify: `internal/cli/app.go`
- Modify: `internal/cli/app_test.go`

- [ ] **Step 1: 写 CLI 参数解析失败测试**

```go
func TestMain_SearchRoutesAndBindsOptions(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"search", "--limit", "5", "--sort", "stars", "--order", "desc", "--json", "ripgrep", "language:rust", "user:BurntSushi"})
	if err != nil {
		t.Fatalf("expected search command to parse, got %v", err)
	}
	if len(calls) != 1 || calls[0].name != "search" {
		t.Fatalf("unexpected routed call: %#v", calls)
	}
	opts := calls[0].options.(*SearchOptions)
	if opts.Keyword != "ripgrep" {
		t.Fatalf("expected keyword ripgrep, got %q", opts.Keyword)
	}
	if len(opts.Extras) != 2 {
		t.Fatalf("expected two extras, got %#v", opts.Extras)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/cli -run SearchRoutesAndBindsOptions`
Expected: FAIL，提示 `SearchOptions` / `newSearchCmd` 未定义

- [ ] **Step 3: 写最小命令实现并注册**

```go
type SearchOptions struct {
	Keyword string
	Extras  []string
	Limit   int
	Sort    string
	Order   string
	JSON    bool
}

func newSearchCmd(handler CommandHandler) (*capp.Cmd, func()) {
	opts := &SearchOptions{Limit: 10}
	cmd := capp.NewCmd("search", "Search GitHub repositories", func(cmd *capp.Cmd) error {
		args := cmd.RemainArgs()
		if err := validateNoTrailingFlags(cmd); err != nil {
			return err
		}
		if len(args) > 0 {
			opts.Keyword = args[0]
		}
		if len(args) > 1 {
			opts.Extras = append([]string(nil), args[1:]...)
		}
		snapshot := *opts
		return handler(cmd.Name, &snapshot)
	})
	cmd.StringVar(&opts.Sort, "sort", "", "Search sort: stars, updated")
	cmd.StringVar(&opts.Order, "order", "", "Search order: desc, asc")
	cmd.IntVar(&opts.Limit, "limit", 10, "Limit repository count")
	cmd.BoolVar(&opts.JSON, "json", false, "Output as JSON")
	cmd.AddArg("keyword", "Search keyword", false, nil)
	return cmd, func() { *opts = SearchOptions{Limit: 10} }
}
```

并在 `newApp()` 里注册：

```go
app.add(newSearchCmd(handler))
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/cli -run SearchRoutesAndBindsOptions`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/cli/search_cmd.go internal/cli/app.go internal/cli/app_test.go
git commit -m "feat: add search command"
```

## Task 4: 接通 service 层和输出

**Files:**
- Modify: `internal/cli/service.go`
- Modify: `internal/cli/service_test.go`

- [ ] **Step 1: 写 service 失败测试**

```go
func TestHandleSearchPrintsTable(t *testing.T) {
	svc := &cliService{
		searchService: app.SearchService{
			Client: &fakeSearchClientForCLI{
				result: app.SearchResult{
					Query: "ripgrep language:rust",
					TotalCount: 1,
					Items: []app.SearchRepo{{FullName: "BurntSushi/ripgrep", Language: "Rust", Stars: 10}},
				},
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handle("search", &SearchOptions{Keyword: "ripgrep", Extras: []string{"language:rust"}})
	if err != nil {
		t.Fatalf("handle search: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "BurntSushi/ripgrep") {
		t.Fatalf("expected repo row, got %q", got)
	}
}

func TestHandleSearchPrintsJSON(t *testing.T) {
	// 验证 --json 直接输出 SearchResult JSON
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/cli -run HandleSearch`
Expected: FAIL，提示 `searchService` 或 `handle search` 未实现

- [ ] **Step 3: 写最小 wiring 和输出实现**

在 `cliService` 中新增字段：

```go
searchService app.SearchService
```

在 `newCLIService()` 中初始化：

```go
searchService := app.SearchService{Client: newGitHubSearchClient(defaultOpts)}
```

在 `handle()` 中新增分支：

```go
case "search":
	opts := options.(*SearchOptions)
	return s.handleSearch(opts)
```

新增处理函数：

```go
func (s *cliService) handleSearch(opts *SearchOptions) error {
	result, err := s.searchService.Search(app.SearchOptions{
		Keyword: opts.Keyword,
		Extras:  append([]string(nil), opts.Extras...),
		Limit:   opts.Limit,
		Sort:    opts.Sort,
		Order:   opts.Order,
		JSON:    opts.JSON,
	})
	if err != nil {
		return err
	}
	if opts.JSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	printSearchResult(result)
	return nil
}

func printSearchResult(result app.SearchResult) {
	cols := []string{"Full Name", "Language", "Stars", "Updated", "Description"}
	rows := make([][]any, 0, len(result.Items))
	for _, item := range result.Items {
		rows = append(rows, []any{item.FullName, item.Language, item.Stars, item.UpdatedAt.Format(time.RFC3339), item.Description})
	}
	ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/cli -run HandleSearch`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/cli/service.go internal/cli/service_test.go
git commit -m "feat: wire github search command"
```

## Task 5: 全量验证与收尾

**Files:**
- Modify: 如上所有相关文件

- [ ] **Step 1: 跑聚焦测试**

Run: `go test ./internal/app ./internal/cli`
Expected: PASS

- [ ] **Step 2: 跑全量测试**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 3: 检查 diff**

Run: `git diff --stat`
Expected: 仅包含 search 命令相关文件改动

- [ ] **Step 4: 提交最终收尾**

```bash
git add internal/app/search.go internal/app/search_test.go internal/cli/search_cmd.go internal/cli/search_client.go internal/cli/search_client_test.go internal/cli/app.go internal/cli/app_test.go internal/cli/service.go internal/cli/service_test.go
git commit -m "feat: add github repository search command"
```
