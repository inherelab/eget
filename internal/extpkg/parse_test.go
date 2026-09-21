package extpkg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func TestParseListNPMFromFixture(t *testing.T) {
	manager := Manager{Name: "npm", Parser: ParserNPMJSON}
	packages, err := ParseList(manager, CommandResult{Stdout: readFixture(t, "npm-ls.json")})
	assert.NoErr(t, err)
	assert.Eq(t, 2, len(packages))
	assert.Eq(t, "@colbymchenry/codegraph", packages[0].Name)
	assert.Eq(t, "0.9.4", packages[0].Version)
	assert.Eq(t, "npm", packages[0].Manager)
	assert.Eq(t, "command-code", packages[1].Name)
	assert.Eq(t, "1.58.1", packages[1].Version)
}

func TestParseOutdatedNPMFromFixture(t *testing.T) {
	manager := Manager{Name: "npm", Parser: ParserNPMJSON}
	// npm exits 1 when it finds outdated packages; that is not a failure.
	res := CommandResult{Stdout: readFixture(t, "npm-outdated.json"), ExitCode: 1}
	packages, err := ParseOutdated(manager, res)
	assert.NoErr(t, err)
	assert.Eq(t, 2, len(packages))
	assert.Eq(t, "agent-browser", packages[0].Name)
	assert.Eq(t, "0.27.2", packages[0].Version)
	assert.Eq(t, "0.38.1", packages[0].Latest)
}

func TestParseOutdatedNPMEmptyObject(t *testing.T) {
	manager := Manager{Name: "npm", Parser: ParserNPMJSON}
	packages, err := ParseOutdated(manager, CommandResult{Stdout: []byte("{}\n")})
	assert.NoErr(t, err)
	assert.Eq(t, 0, len(packages))
}

func TestParseListNPMReportsStdoutError(t *testing.T) {
	manager := Manager{Name: "npm", Parser: ParserNPMJSON}
	// npm writes errors to stdout and exits 1.
	res := CommandResult{Stdout: readFixture(t, "npm-error-stdout.txt"), ExitCode: 1}
	_, err := ParseList(manager, res)
	if err == nil {
		t.Fatal("expected parse error for npm error output")
	}
	assert.Contains(t, err.Error(), "Unknown command")
	assert.Contains(t, err.Error(), "exit 1")
}

func TestParseListPNPMReadsTopLevelArray(t *testing.T) {
	manager := Manager{Name: "pnpm", Parser: ParserPNPMJSON}

	t.Run("empty global store", func(t *testing.T) {
		packages, err := ParseList(manager, CommandResult{Stdout: readFixture(t, "pnpm-ls.json")})
		assert.NoErr(t, err)
		assert.Eq(t, 0, len(packages))
	})

	t.Run("populated dependencies", func(t *testing.T) {
		// Not a measured fixture: this machine has no pnpm global packages and
		// `pnpm add -g` refuses until its global bin dir is on PATH. The shape
		// follows the npm-compatible {version: ...} form; see design 4.7.
		body := []byte(`[{"path":"C:/pnpm/global/v11","private":true,"dependencies":{"is-number":{"version":"1.0.0"}}}]`)
		packages, err := ParseList(manager, CommandResult{Stdout: body})
		assert.NoErr(t, err)
		assert.Eq(t, 1, len(packages))
		assert.Eq(t, "is-number", packages[0].Name)
		assert.Eq(t, "1.0.0", packages[0].Version)
	})
}

func TestParseOutdatedPNPMEmptyObject(t *testing.T) {
	manager := Manager{Name: "pnpm", Parser: ParserPNPMJSON}
	packages, err := ParseOutdated(manager, CommandResult{Stdout: []byte("{}\n")})
	assert.NoErr(t, err)
	assert.Eq(t, 0, len(packages))
}

func TestParseListUVToolText(t *testing.T) {
	manager := Manager{Name: "uv", Parser: ParserUVToolText}
	packages, err := ParseList(manager, CommandResult{Stdout: readFixture(t, "uv-tool-list.txt")})
	assert.NoErr(t, err)
	assert.Eq(t, 2, len(packages))
	assert.Eq(t, "graphifyy", packages[0].Name)
	assert.Eq(t, "0.8.18", packages[0].Version)
	// The executable name differs from the tool name; the tree lines are skipped.
	assert.Eq(t, "semble", packages[1].Name)
	assert.Eq(t, "0.2.0", packages[1].Version)
}

func TestParseOutdatedUVToolTextReadsLatest(t *testing.T) {
	manager := Manager{Name: "uv", Parser: ParserUVToolText}
	packages, err := ParseOutdated(manager, CommandResult{Stdout: readFixture(t, "uv-tool-outdated.txt")})
	assert.NoErr(t, err)
	assert.Eq(t, 2, len(packages))
	assert.Eq(t, "graphifyy", packages[0].Name)
	assert.Eq(t, "0.8.18", packages[0].Version)
	assert.Eq(t, "0.9.65", packages[0].Latest)
	assert.Eq(t, "0.6.0", packages[1].Latest)
}

func TestParseListPipxJSON(t *testing.T) {
	manager := Manager{Name: "pipx", Parser: ParserPipxJSON}

	t.Run("empty", func(t *testing.T) {
		body := []byte(`{"pipx_spec_version":"0.1","venvs":{}}`)
		packages, err := ParseList(manager, CommandResult{Stdout: body})
		assert.NoErr(t, err)
		assert.Eq(t, 0, len(packages))
	})

	t.Run("filled uses package_version", func(t *testing.T) {
		packages, err := ParseList(manager, CommandResult{Stdout: readFixture(t, "pipx-list-filled.json")})
		assert.NoErr(t, err)
		assert.Eq(t, 1, len(packages))
		assert.Eq(t, "pycowsay", packages[0].Name)
		assert.Eq(t, "0.0.0.1", packages[0].Version)
	})
}

func TestParseOutdatedPipxJSONEnvelope(t *testing.T) {
	manager := Manager{Name: "pipx", Parser: ParserPipxJSON}

	t.Run("empty", func(t *testing.T) {
		body := []byte(`{"command":["list"],"data":{"packages":[],"packages_checked":0,"skipped":[]},"errors":[],"status":"success"}`)
		packages, err := ParseOutdated(manager, CommandResult{Stdout: body})
		assert.NoErr(t, err)
		assert.Eq(t, 0, len(packages))
	})

	t.Run("filled uses version and latest_version", func(t *testing.T) {
		packages, err := ParseOutdated(manager, CommandResult{Stdout: readFixture(t, "pipx-outdated-filled.json")})
		assert.NoErr(t, err)
		assert.Eq(t, 1, len(packages))
		assert.Eq(t, "pycowsay", packages[0].Name)
		assert.Eq(t, "0.0.0.1", packages[0].Version)
		assert.Eq(t, "0.0.0.2", packages[0].Latest)
	})

	t.Run("envelope failure is an error even with exit 0", func(t *testing.T) {
		body := []byte(`{"data":{"packages":[]},"errors":["boom"],"status":"error"}`)
		_, err := ParseOutdated(manager, CommandResult{Stdout: body})
		if err == nil {
			t.Fatal("expected error for pipx envelope failure")
		}
		assert.Contains(t, err.Error(), "boom")
	})
}

func TestParseListCargoText(t *testing.T) {
	manager := Manager{Name: "cargo", Parser: ParserCargoText}

	t.Run("empty output is an empty list", func(t *testing.T) {
		packages, err := ParseList(manager, CommandResult{Stdout: []byte("")})
		assert.NoErr(t, err)
		assert.Eq(t, 0, len(packages))
	})

	t.Run("filled", func(t *testing.T) {
		packages, err := ParseList(manager, CommandResult{Stdout: readFixture(t, "cargo-list-filled.txt")})
		assert.NoErr(t, err)
		assert.Eq(t, 1, len(packages))
		assert.Eq(t, "cargotest", packages[0].Name)
		assert.Eq(t, "0.1.0", packages[0].Version)
	})
}

func TestParseListBunText(t *testing.T) {
	manager := Manager{Name: "bun", Parser: ParserBunText}

	t.Run("filled tree", func(t *testing.T) {
		packages, err := ParseList(manager, CommandResult{Stdout: readFixture(t, "bun-ls-filled.txt")})
		assert.NoErr(t, err)
		assert.Eq(t, 1, len(packages))
		assert.Eq(t, "is-number", packages[0].Name)
		assert.Eq(t, "1.0.0", packages[0].Version)
	})

	t.Run("missing package.json means no packages", func(t *testing.T) {
		res := CommandResult{Stderr: readFixture(t, "bun-ls-empty.err"), ExitCode: 1}
		packages, err := ParseList(manager, res)
		assert.NoErr(t, err)
		assert.Eq(t, 0, len(packages))
	})
}

func TestParseOutdatedBunText(t *testing.T) {
	manager := Manager{Name: "bun", Parser: ParserBunText}

	t.Run("filled table", func(t *testing.T) {
		packages, err := ParseOutdated(manager, CommandResult{Stdout: readFixture(t, "bun-outdated-filled.txt")})
		assert.NoErr(t, err)
		assert.Eq(t, 1, len(packages))
		assert.Eq(t, "is-number", packages[0].Name)
		assert.Eq(t, "1.0.0", packages[0].Version)
		assert.Eq(t, "7.0.0", packages[0].Latest)
	})

	t.Run("banner only means no packages", func(t *testing.T) {
		res := CommandResult{
			Stdout:   []byte("bun outdated v1.4.2 (744846f84)\n"),
			Stderr:   []byte("error: missing package.json, nothing outdated\n"),
			ExitCode: 1,
		}
		packages, err := ParseOutdated(manager, res)
		assert.NoErr(t, err)
		assert.Eq(t, 0, len(packages))
	})
}

func TestParseLinesRegex(t *testing.T) {
	manager := Manager{
		Name:          "scoop",
		Parser:        ParserLinesRegex,
		OutdatedRegex: `^(?P<name>\S+)\s+(?P<version>\S+)\s+(?P<latest>\S+)$`,
	}
	body := []byte("scoop 1.0.0 2.0.0\nnot-a-match\nother 3.0.0 4.0.0\n")
	packages, err := ParseOutdated(manager, CommandResult{Stdout: body})
	assert.NoErr(t, err)
	assert.Eq(t, 2, len(packages))
	assert.Eq(t, "other", packages[0].Name)
	assert.Eq(t, "3.0.0", packages[0].Version)
	assert.Eq(t, "4.0.0", packages[0].Latest)
	assert.Eq(t, "scoop", packages[1].Name)
}

func TestParseLinesRegexRequiresNameGroup(t *testing.T) {
	manager := Manager{Name: "scoop", Parser: ParserLinesRegex, ListRegex: `^(?P<pkg>\S+)$`}
	_, err := ParseList(manager, CommandResult{Stdout: []byte("a\n")})
	if err == nil {
		t.Fatal("expected error for regex without a name group")
	}
	assert.Contains(t, err.Error(), "name")
}

func TestParseUnknownParser(t *testing.T) {
	_, err := ParseList(Manager{Name: "x", Parser: "nope"}, CommandResult{Stdout: []byte("{}")})
	if err == nil {
		t.Fatal("expected error for unknown parser")
	}
	assert.Contains(t, err.Error(), "unknown parser")
}

func TestOutputErrorIncludesBothStreams(t *testing.T) {
	manager := Manager{Name: "pipx"}
	err := outputError(manager, CommandResult{
		Stdout:   []byte("out-line"),
		Stderr:   []byte("err-line"),
		ExitCode: 3,
	}, "boom")
	message := err.Error()
	if !strings.Contains(message, "out-line") || !strings.Contains(message, "err-line") {
		t.Fatalf("expected both streams in %q", message)
	}
	assert.Contains(t, message, "exit 3")
}
