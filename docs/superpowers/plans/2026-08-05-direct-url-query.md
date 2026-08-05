# Direct URL Query Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let `eget query` inspect HTTP(S) file metadata without downloading the complete file.

**Architecture:** Add one HTTP probe in `internal/client`, route direct URL targets through the existing app query service, and extend the existing text/JSON renderer. Reuse the current HTTP options, redirect handling, response filename parser, and query command; add no dependency or parallel command.

**Tech Stack:** Go standard library HTTP, existing gcli command layer, gookit assertions, httptest.

---

### Task 1: Probe direct URL response metadata

**Files:**
- Create: `internal/client/url_query.go`
- Create: `internal/client/url_query_test.go`

- [x] **Step 1: Write failing HEAD metadata tests**

Define tests with `httptest.Server` for `QueryURL(rawURL string, opts Options) (URLInfo, error)`. Assert requested/final URL, redirect handling, status, `Content-Disposition` filename, byte size, content type, last-modified, ETag, and accept-ranges. Add an invalid-scheme case for `ftp://example.com/file.zip`.

- [x] **Step 2: Run the tests and verify RED**

Run: `go test ./internal/client -run TestQueryURL -count=1`

Expected: build failure because `QueryURL` and `URLInfo` do not exist.

- [x] **Step 3: Implement the minimal HEAD probe**

Create this public result shape:

```go
type URLInfo struct {
	RequestedURL string `json:"requested_url" mapstructure:"requested_url"`
	FinalURL string `json:"final_url" mapstructure:"final_url"`
	Status string `json:"status" mapstructure:"status"`
	StatusCode int `json:"status_code" mapstructure:"status_code"`
	FileName string `json:"file_name,omitempty" mapstructure:"file_name"`
	Size *int64 `json:"size,omitempty" mapstructure:"size"`
	ContentType string `json:"content_type,omitempty" mapstructure:"content_type"`
	LastModified string `json:"last_modified,omitempty" mapstructure:"last_modified"`
	ETag string `json:"etag,omitempty" mapstructure:"etag"`
	AcceptRanges string `json:"accept_ranges,omitempty" mapstructure:"accept_ranges"`
}
```

Validate the scheme as HTTP or HTTPS, call `requestWithOptions(http.MethodHead, rawURL, "", opts)`, close the body, and populate the result from the final response. Reuse `responseFilename`.

- [x] **Step 4: Add and verify range fallback tests**

Add a server that returns `405` for HEAD and `206` for `GET Range: bytes=0-0`, plus a server whose successful HEAD omits size. Assert the fallback request contains the range header, total size comes from `Content-Range`, and only response headers are used.

Run: `go test ./internal/client -run TestQueryURL -count=1`

Expected before fallback implementation: FAIL because size is unknown or GET was not called.

- [x] **Step 5: Implement minimal fallback and verify GREEN**

Fallback when HEAD returns `405`/`501` or has no usable content length. Issue `GET` with `bytes=0-0`, parse the total after `/` in `Content-Range`, close the body without reading it, and otherwise use the response content length. Preserve valid non-success HEAD responses rather than turning missing optional headers into errors.

Run: `go test ./internal/client -run TestQueryURL -count=1`

Expected: PASS.

- [x] **Step 6: Commit Task 1**

```shell
git add internal/client/url_query.go internal/client/url_query_test.go
git commit -m "feat(query): probe direct URL metadata, refs #52"
```

### Task 2: Route URL targets through the existing query service

**Files:**
- Modify: `internal/app/query.go`
- Modify: `internal/app/query_test.go`
- Modify: `internal/cli/wiring.go`
- Modify: `internal/cli/query_cmd.go`
- Modify: `internal/cli/app_query_search_test.go`

- [x] **Step 1: Write failing app routing tests**

Add tests that inject `QueryService.URLInfo`, query an HTTP URL with the default `latest` action, and expect `Action == "info"` plus populated `URLInfo`. Add table cases rejecting `releases` and `assets`; retain `info` as an accepted explicit action. Assert a repository query still calls the existing release client.

- [x] **Step 2: Run the tests and verify RED**

Run: `go test ./internal/app -run 'TestQueryService(DirectURL|LatestUsesDefaultAction)' -count=1`

Expected: build failure because the service/result have no URL metadata field.

- [x] **Step 3: Implement URL routing**

Add `type QueryURLInfo = client.URLInfo`, `URLInfo *QueryURLInfo` to `QueryResult`, and `URLInfo func(string) (QueryURLInfo, error)` to `QueryService`. Before SourceForge/repository normalization, detect `install.TargetDirectURL`; accept the default `latest` and explicit `info`, call the probe, and return action `info`. Reject release-only actions with a target-specific error.

- [x] **Step 4: Wire the probe and update command help**

Set the service callback in `newCLIService`:

```go
URLInfo: func(rawURL string) (client.URLInfo, error) {
	return client.QueryURL(rawURL, defaultClientOpts)
},
```

Update the query target help and examples to include `eget query https://example.com/tool.zip` and `--json`. Add a parser test proving the URL reaches `QueryOptions.Target` unchanged.

- [x] **Step 5: Verify Task 2 GREEN**

Run: `go test ./internal/app ./internal/cli -run 'Test(QueryServiceDirectURL|QueryServiceLatestUsesDefaultAction|Main_Query)' -count=1`

Expected: PASS.

- [x] **Step 6: Commit Task 2**

```shell
git add internal/app/query.go internal/app/query_test.go internal/cli/wiring.go internal/cli/query_cmd.go internal/cli/app_query_search_test.go
git commit -m "feat(query): route direct URL queries, refs #52"
```

### Task 3: Render URL metadata as text and JSON

**Files:**
- Modify: `internal/cli/render/render.go`
- Modify: `internal/cli/query_search_handler_test.go`

- [x] **Step 1: Write failing rendering tests**

Add a text test expecting requested URL, final URL, status, filename, human-readable size, content type, last-modified, ETag, and accept-ranges. Add a JSON test that unmarshals output and verifies raw byte size under `url_info.size`; add an unknown-size case proving the size key is omitted.

- [x] **Step 2: Run the tests and verify RED**

Run: `go test ./internal/cli -run 'Test(PrintQueryResultURLInfo|QueryResultJSONURLInfo)' -count=1`

Expected: FAIL because the renderer ignores `URLInfo`.

- [x] **Step 3: Implement minimal text and JSON rendering**

Add `URLInfo *app.QueryURLInfo` to `queryResultDisplay`, copy it in `queryResultToDisplay`, and handle it before repository output in `PrintQueryResult`. Use the existing `mathutil.DataSize` for known byte size and `unknown` for text when size is absent. Keep JSON size as raw bytes.

- [x] **Step 4: Verify Task 3 GREEN**

Run: `go test ./internal/cli -run 'Test(PrintQueryResultURLInfo|QueryResultJSONURLInfo)' -count=1`

Expected: PASS.

- [x] **Step 5: Commit Task 3**

```shell
git add internal/cli/render/render.go internal/cli/query_search_handler_test.go
git commit -m "feat(query): render direct URL metadata, refs #52"
```

### Task 4: Final verification

**Files:**
- Modify: `docs/superpowers/plans/2026-08-05-direct-url-query.md`

- [x] **Step 1: Format and run focused package tests**

Run: `gofmt -w internal/client/url_query.go internal/client/url_query_test.go internal/app/query.go internal/app/query_test.go internal/cli/wiring.go internal/cli/query_cmd.go internal/cli/app_query_search_test.go internal/cli/render/render.go internal/cli/query_search_handler_test.go`

Run: `go test ./internal/client ./internal/app ./internal/cli -count=1`

Expected: PASS.

- [x] **Step 2: Run the full repository gate**

Run: `go test ./... -count=1`

Expected: PASS.

- [x] **Step 3: Check diff scope**

Run: `git diff --check`

Run: `npx gitnexus detect-changes -r eget --scope all`

Expected: only the direct URL probe, query routing, wiring, help, renderer, tests, and this plan are affected.

- [x] **Step 4: Mark every plan checkbox complete and commit the plan update**

```shell
git add docs/superpowers/plans/2026-08-05-direct-url-query.md
git commit -m "docs: complete direct URL query plan, refs #52"
```
