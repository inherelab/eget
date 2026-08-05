# Direct URL Query Design

## Goal

Extend the existing `query` command so a direct HTTP or HTTPS URL can be inspected without downloading the complete response body.

```shell
eget query https://example.com/tool.zip
eget query --json https://example.com/tool.zip
```

Repository and SourceForge query behavior remains unchanged.

## Scope

For a direct URL, report the metadata exposed by the final HTTP response:

- requested URL and final URL after redirects;
- HTTP status;
- file name from `Content-Disposition`, falling back to the URL path;
- file size from `Content-Length` or `Content-Range` when available;
- `Content-Type`, `Last-Modified`, `ETag`, and `Accept-Ranges`.

Unknown fields remain empty or unknown. HTTP does not expose a reliable file creation time, so no creation-time field is inferred.

The enhancement does not parse HTML download pages, inspect archive contents, or download the complete file.

## Command Behavior

Target kind selects the default query behavior:

- a direct HTTP(S) URL performs URL metadata inspection;
- a repository or SourceForge target keeps the current default `latest` action.

For direct URLs, explicit release actions such as `releases` and `assets` are rejected. `--json` returns the same metadata as the text renderer in a stable structured form.

## Request Flow

1. Send a `HEAD` request using the existing HTTP client settings, headers, proxy configuration, and redirect handling.
2. If `HEAD` is unsupported or does not expose a usable size, send `GET` with `Range: bytes=0-0`.
3. Read response headers only and close the body immediately.
4. Derive the file name and size without persisting a download.

A server may omit or misreport metadata. The command reports only confirmed response values and does not manufacture missing values.

## Code Boundaries

- `internal/client` owns HTTP probing and response-header parsing.
- `internal/app` detects direct URL query targets and adds URL metadata to the existing query result.
- `internal/cli` keeps the existing command and renders text or JSON output.

No new command, dependency, configuration section, or persistence format is introduced.

## Error Handling

- Invalid or non-HTTP(S) direct URLs are rejected by existing target validation.
- A transport failure is returned with its underlying context.
- If both `HEAD` and range probing fail, the final probe error is returned.
- A valid HTTP response is reported with its status; missing optional headers are not errors.

## Verification

Focused tests cover:

- successful `HEAD` metadata extraction and redirect final URL;
- file-name precedence and unknown optional fields;
- range fallback when `HEAD` is unsupported or lacks size;
- text and JSON rendering;
- direct URL routing without changing repository query behavior;
- proof that the fallback does not consume the complete response body.

The final gate is `go test ./...` plus GitNexus change detection.
