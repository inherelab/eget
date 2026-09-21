# Streaming Download Extraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task after the scope is confirmed. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Remove the requirement to keep an entire downloaded asset in a []byte while preserving checksum verification, ZIP/TAR/7z extraction, cache behavior, local-file targets, and existing CLI output.

**Architecture:** Downloads will resolve to a seekable on-disk source: an existing local file, a stable cache file, or a temporary file for uncached remote downloads. Verification and extraction will reopen or seek this source instead of receiving []byte; archive readers will consume io.ReadSeeker plus size, and archive entries will stream directly to their destination. The existing mislabeled TAR fallback from issue #58 remains in place and will reset the seek position before probing TAR.

**Tech Stack:** Go standard library (os.File, io.Reader, io.ReadSeeker, io.Copy, crypto/sha256), existing archive packages, existing Go test suite. No new dependency.

## Global Constraints

- Do not change CLI flags, target selection, checksum messages, extraction paths, or cache naming semantics.
- Preserve the issue #58 fix: a valid TAR payload with a .zip filename remains extractable; malformed data still returns an error.
- Keep go test ./... as the final required verification.
- Do not add a memory threshold or configurable buffering knob before profiling demonstrates a need.
- Do not keep a compatibility []byte path for the full asset in the production download flow; test fixtures may continue to use bytes.Reader.
- This crosses download, verification, extraction, and temporary-file lifecycle code, so implementation requires explicit scope confirmation before editing production code.

## Current Data Flow and Problem Boundary

The current path is:

~~~text
DownloadWithResult → bytes.Buffer / cache file → os.ReadFile → downloadBodyResult.Body []byte
                 → Verifier.Verify([]byte)
                 → Extractor.Extract([]byte) / ExtractAllTo([]byte)
~~~

The avoidable allocations are:

1. A cache download is written to disk and immediately read back in full.
2. A no-cache download is accumulated in bytes.Buffer; parallel range download first creates a full body buffer and then copies it into that buffer.

ZIP and 7z need io.ReaderAt plus size, while TAR needs a sequential reader. A seekable *os.File satisfies the required operations.

## Planned File Map

- Modify internal/install/runner_download.go: return a disk-backed download source and clean up only temporary files.
- Modify internal/install/runner_extract.go: replace Body []byte with source path/size metadata.
- Modify internal/install/runner.go: verify and extract by opening/rewinding the source; defer cleanup on every exit path.
- Modify internal/install/service.go and internal/install/verifiers.go: change verification from []byte to io.Reader and hash incrementally.
- Modify internal/install/extractor.go and internal/install/archive.go: pass a seekable reader and size through extractor APIs; stream selected entries when materializing them.
- Modify internal/install/archive_formats.go: construct ZIP/TAR/7z readers from io.ReadSeeker; preserve and seek-reset the .zip-named TAR fallback.
- Modify internal/install/system7z.go: pass the downloaded file path to 7z instead of writing a second full temporary byte copy where the platform extractor is selected.
- Modify internal/install/runner_installer.go and internal/install/runner_run_asset.go: copy from the source stream when a materialized file is required.
- Modify affected internal/install/*_test.go files: add tests for disk-backed download, streaming verification, reader-based archive extraction, cleanup, and the issue #58 regression.
- Update docs/architecture.md only if the final source lifecycle or cache behavior needs user-facing documentation; this plan is the required design record.

### Task 1: Define the disk-backed source contract

**Files:**
- Modify: internal/install/runner_extract.go:9-13
- Modify: internal/install/runner_download.go:22-91
- Test: internal/install/runner_download_cache_test.go

**Interfaces:**

~~~go
type downloadBodyResult struct {
    Path     string
    Size     int64
    ModTime  time.Time
    Filename string
    Temp     bool
}

func (d downloadBodyResult) Open() (*os.File, error)
func (d downloadBodyResult) Close() error
~~~

Path is a local file for local targets and cache hits, or a temporary file for uncached remote downloads. Temp is true only for a temporary source. Close must never remove a user local file or configured cache file.

- [x] Step 1: Write failing tests for local sources, cache hits, uncached temporary sources, and cleanup.
- [x] Step 2: Run the focused tests and confirm the old Body contract fails to compile.
- [x] Step 3: Change downloadBody so cache hits return the cache path without os.ReadFile, cache misses use DownloadFile directly into the cache path, and no-cache downloads use os.CreateTemp plus DownloadWithResult to a file. Stat the completed file for Size. Remove temporary files on every error.
- [x] Step 4: Re-run the focused tests; cache and temporary-source assertions pass.
- [x] Step 5: Include the disk-backed source implementation in the streaming refactor commit.

### Task 2: Stream checksum verification and materialization

**Files:**
- Modify: internal/install/service.go:33-35
- Modify: internal/install/verifiers.go
- Modify: internal/install/runner.go:148-167
- Modify: internal/install/runner_installer.go:58-82
- Modify: internal/install/runner_run_asset.go:26-48
- Test: internal/install/verifiers_test.go, internal/install/runner_run_asset_test.go, internal/install/runner_installer_test.go

**Interfaces:**

~~~go
type Verifier interface {
    Verify(io.Reader) error
}

func (d downloadBodyResult) Verify(v Verifier) error {
    f, err := d.Open()
    if err != nil {
        return err
    }
    defer f.Close()
    return v.Verify(f)
}
~~~

- [x] Step 1: Update verifier tests to use bytes.NewReader and keep materialization on readers.
- [x] Step 2: Run the focused verifier and run-asset tests; old Verify([]byte) call sites fail before migration.
- [x] Step 3: Implement SHA-256 verification with io.Copy(hash, reader). In Run, defer downloaded.Close immediately after download. Use io.Copy from downloaded.Open for run-asset and installer materialization.
- [x] Step 4: Re-run focused tests with unchanged checksum output.
- [x] Step 5: Include streaming verification in the streaming refactor commit.

### Task 3: Move archive extractors to seekable sources

**Files:**
- Modify: internal/install/extractor.go
- Modify: internal/install/archive.go
- Modify: internal/install/archive_formats.go
- Modify: internal/install/system7z.go
- Test: internal/install/defaults_archive_test.go, internal/install/runner_extract_test.go, internal/install/system7z_test.go

**Interfaces:**

~~~go
type ArchiveFn func(io.ReadSeeker, int64, DecompFn) (Archive, error)

type Extractor interface {
    Extract(io.ReadSeeker, int64, bool) (ExtractedFile, []ExtractedFile, error)
}

type DirectAllExtractor interface {
    ExtractAllTo(io.ReadSeeker, int64, string) ([]string, error)
}
~~~

- [x] Step 1: Add file-backed ZIP/TAR coverage, seek-reset coverage for issue #58, and a direct-all test proving ordinary entries use WriteTo/io.Copy rather than ReadAll.
- [x] Step 2: Run the focused archive tests and migrate the old []byte API call sites.
- [x] Step 3: Construct ZIP, TAR, and 7z readers from io.ReadSeeker plus size. Reset before TAR fallback and parser passes. Selected files and single-file compressed assets stream to their destinations.
- [x] Step 4: Make System7zExtractor pass the source path when available and retain temporary-file fallback for non-file test readers.
- [x] Step 5: Focused tests pass for ZIP, TAR, mislabeled TAR, 7z, strip-components, links, and timestamps.
- [x] Step 6: Include seekable archive extraction in the streaming refactor commit.

### Task 4: Connect Run and preserve all target modes

**Files:**
- Modify: internal/install/runner.go
- Modify: internal/install/runner_extract.go
- Modify: internal/install/runner_download.go
- Modify: internal/install/runner_run_asset.go
- Modify: internal/install/runner_installer.go
- Modify: internal/sdk/install_service.go
- Test: internal/install/runner_test.go, internal/install/runner_run_asset_test.go, internal/install/runner_extract_test.go, internal/install/runner_download_cache_test.go

- [x] Step 1: Existing integration coverage exercises uncached extract-all cleanup, checksum failure, local files, cache extraction, download-only, run-asset, selected-file extraction, and GUI installer materialization.
- [x] Step 2: Run the integration-focused tests and migrate the old Body []byte path.
- [x] Step 3: After download succeeds, defer downloaded.Close. Open fresh file handles for checksum verification and extraction.
- [x] Step 4: Integration tests pass with unchanged result structures and CLI messages.
- [x] Step 5: Include the run and SDK source migration in the streaming refactor commit.

### Task 5: Full verification and memory regression check

**Files:**
- Test: affected internal/install/*_test.go and internal/client/*_test.go
- Review: docs/architecture.md

- [x] Step 1: Run gofmt, go vet ./..., and git diff --check; all pass.
- [x] Step 2: Run go test ./...; every package passes.
- [x] Step 3: Run an equivalent 16 MiB ZIP smoke with no cache and with a cache directory; both extract-all runs passed and the extracted payload matched.
- [x] Step 4: Inspect the successful asset path. Production code no longer reads the downloaded asset into []byte; remaining io.ReadAll calls are archive compatibility methods or small checksum responses.
- [x] Step 5: Run npx gitnexus detect-changes --scope unstaged --repo eget; it reported the planned 21 files and 40 affected execution flows, with the interface migration as the risk boundary.
- [x] Step 6: Commit the implementation and plan updates as `c36212d` and push `master` to `origin`.

## Acceptance Criteria

- Successful remote downloads never materialize the complete asset as downloadBodyResult.Body []byte.
- Checksum verification reads from a stream and has the same pass/fail behavior and CLI output.
- ZIP, TAR, 7z, compressed TAR, local files, cache hits, uncached downloads, selected-file extraction, extract-all, download-only, run-asset, and GUI installer flows remain functional.
- Issue #58's mislabeled TAR asset still extracts successfully.
- Temporary files are removed on success and all error exits; local and configured cache files are preserved.
- go test ./..., go vet ./..., and git diff --check pass.

## Deliberately Deferred

- Memory-mapping archives: unnecessary until file-backed io.ReadSeeker is measured as a bottleneck.
- Streaming directly over a non-seekable HTTP response: archive selection, checksum verification, and multi-pass extraction require seekability; a temp file is the simpler reliable boundary.
- Exact RSS benchmarks in CI: platform-dependent and prone to false failures; use allocation profiles or manual large-asset smoke checks if later evidence shows a regression.
