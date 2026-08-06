# `--file` Regular Expression Filter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `REG:` include and `^REG:` exclude expressions to archive-entry selection through `--file` while preserving existing glob behavior.

**Architecture:** Keep the existing `Chooser` pipeline and add one small regex-backed implementation beside `GlobChooser`. Route each comma-separated file expression through a shared chooser factory, so regex and glob entries participate in the existing include/exclude precedence without changing CLI option binding or extraction code.

**Tech Stack:** Go standard library `regexp`, existing `Chooser` interface, gookit `assert`, Go unit tests.

---

## File Structure

- Modify `internal/install/chooser.go`: parse `REG:` entries and match them against normalized archive paths and basenames.
- Modify `internal/install/chooser_test.go`: prove regex include/exclude, mixed expressions, implicit include-all, and validation behavior.
- Modify `README.md`: document the English command syntax and one example.
- Modify `README.zh-CN.md`: document the Chinese command syntax and one example.
- Modify `AGENTS.md`: track the plan while it is active, then remove the completed work item.

### Task 1: Add failing chooser behavior tests

**Files:**
- Modify: `internal/install/chooser_test.go`

- [x] **Step 1: Add failing tests for regex matching and validation**

Add table-driven tests using the existing `assert` package:

```go
func TestFileChooserSupportsRegexPatterns(t *testing.T) {
	tests := []struct {
		expr string
		name string
		want bool
	}{
		{expr: `REG:(?i)\.(hlf|lng)$`, name: `docs\\English.HLF`, want: true},
		{expr: `REG:^plugins/.+\.dll$`, name: `plugins\\foo\\bar.dll`, want: true},
		{expr: `REG:(?i)\.(hlf|lng)$`, name: `docs\\readme.txt`, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.expr+" "+tt.name, func(t *testing.T) {
			chooser, err := NewFileChooser(tt.expr)
			assert.NoErr(t, err)
			_, possible := chooser.Choose(tt.name, false, 0)
			assert.Eq(t, tt.want, possible)
		})
	}
}

func TestFileChooserSupportsRegexExcludePatterns(t *testing.T) {
	chooser, err := NewFileChooser(`*.exe,^REG:(?i)(^|/)x86/`)
	assert.NoErr(t, err)

	_, possible := chooser.Choose(`bin\\x64\\tool.exe`, false, 0)
	assert.True(t, possible)
	_, possible = chooser.Choose(`bin\\x86\\tool.exe`, false, 0)
	assert.False(t, possible)
}

func TestFileChooserRegexExcludeOnlyDefaultsToAllFiles(t *testing.T) {
	chooser, err := NewFileChooser(`^REG:(?i)\.(map|cmd|md|txt|diz)$`)
	assert.NoErr(t, err)

	_, possible := chooser.Choose(`bin\\Far.exe`, false, 0)
	assert.True(t, possible)
	_, possible = chooser.Choose(`docs\\README.md`, false, 0)
	assert.False(t, possible)
}

func TestFileChooserRejectsInvalidRegex(t *testing.T) {
	for _, expr := range []string{`REG:`, `^REG:`, `REG:[`} {
		t.Run(expr, func(t *testing.T) {
			_, err := NewFileChooser(expr)
			assert.Err(t, err)
		})
	}
}
```

- [x] **Step 2: Run the focused tests and verify RED**

Run:

```powershell
go test ./internal/install -run 'TestFileChooser.*Regex' -count=1
```

Expected: FAIL because `REG:` is still compiled as a glob or accepted as an empty literal rather than parsed as a regular expression.

- [x] **Step 3: Commit the red tests**

```powershell
git add -- internal/install/chooser_test.go
git commit -m "test(file): define regex filter behavior, refs #53"
```

### Task 2: Implement the minimal regex chooser

**Files:**
- Modify: `internal/install/chooser.go`
- Test: `internal/install/chooser_test.go`

- [x] **Step 1: Run GitNexus upstream impact analysis**

Run:

```powershell
npx gitnexus impact NewFileChooser -r eget
npx gitnexus impact Choose -r eget
```

Expected: file selection and archive extraction callers only. Stop and warn before editing if risk is HIGH or CRITICAL.

- [x] **Step 2: Add the regex-backed chooser and shared parser**

Add `regexp` to the imports and implement:

```go
type RegexChooser struct {
	re   *regexp.Regexp
	expr string
}

func newFilePatternChooser(expr string) (Chooser, error) {
	if !strings.HasPrefix(expr, "REG:") {
		return NewGlobChooser(expr)
	}
	expr = strings.TrimSpace(strings.TrimPrefix(expr, "REG:"))
	if expr == "" {
		return nil, fmt.Errorf("empty file regex expression")
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid file regex %q: %w", expr, err)
	}
	return &RegexChooser{re: re, expr: expr}, nil
}

func (r *RegexChooser) Choose(name string, dir bool, mode fs.FileMode) (bool, bool) {
	name = archivePathForCompare(name)
	return false, r.re.MatchString(path.Base(name)) || r.re.MatchString(name)
}
```

Replace the three `NewGlobChooser` calls used for user-supplied entries in `NewFileChooser` and `newFilterChooser` with `newFilePatternChooser`. Keep the implicit include-all chooser as `NewGlobChooser("*")`.

- [x] **Step 3: Run focused tests and verify GREEN**

Run:

```powershell
go test ./internal/install -run 'TestFileChooser' -count=1
```

Expected: PASS, including existing glob behavior and all new regex cases.

- [x] **Step 4: Run the install package tests**

Run:

```powershell
go test ./internal/install -count=1
```

Expected: PASS.

- [x] **Step 5: Commit the implementation**

Before committing, run:

```powershell
npx gitnexus detect-changes --scope all -r eget
git diff --check
```

Then commit:

```powershell
git add -- internal/install/chooser.go
git commit -m "feat(file): support regex filters, refs #53"
```

### Task 3: Document and verify the user-facing syntax

**Files:**
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `AGENTS.md`
- Modify: `docs/superpowers/plans/2026-08-06-file-regex-filter.md`

- [x] **Step 1: Extend the `--file` documentation**

Document the following forms in both READMEs:

```text
REG:<expr>     regular-expression include
^REG:<expr>    regular-expression exclude
```

Include this example:

```powershell
eget dl --file "^REG:(?i)\.(map|cmd|md|txt|diz)$" FarGroup/FarManager
```

State that regex uses Go syntax, matches both normalized archive paths and basenames, and remains comma-separated like existing glob filters.

- [x] **Step 2: Run complete verification**

Run:

```powershell
gofmt -w internal/install/chooser.go internal/install/chooser_test.go
go test ./... -count=1
git diff --check
npx gitnexus detect-changes --scope all -r eget
```

Expected: all tests PASS, no whitespace errors, and affected flows stay limited to file selection plus download/install extraction.

- [x] **Step 3: Mark the plan complete and clear active work**

Change every plan checkbox to `[x]` and remove the `--file` regex work item from `AGENTS.md`.

- [ ] **Step 4: Commit documentation and completion state**

```powershell
git add -- README.md README.zh-CN.md AGENTS.md docs/superpowers/plans/2026-08-06-file-regex-filter.md
git commit -m "docs: document file regex filters, refs #53"
```

- [ ] **Step 5: Verify the final branch state**

Run:

```powershell
git status --short
git log -4 --oneline
```

Expected: clean working tree with design, tests, implementation, and documentation recorded in focused commits.
