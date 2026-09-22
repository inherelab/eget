package extpkg

import (
	"fmt"
	"time"
)

// Parser identifiers for built-in manager output formats. Each manager names
// one parser for list output and reuses it for outdated output.
const (
	ParserNPMJSON    = "npm-json"
	ParserPNPMJSON   = "pnpm-json"
	ParserPipxJSON   = "pipx-json"
	ParserUVToolText = "uv-tool-text"
	ParserCargoText  = "cargo-text"
	ParserBunText    = "bun-text"
	ParserScoopTable = "scoop-table"
	ParserLinesRegex = "lines-regex"
)

// DefaultTimeout bounds one manager command.
const DefaultTimeout = 60 * time.Second

// Package is one package installed by an external manager.
type Package struct {
	Manager string
	Name    string
	Version string
	Latest  string
}

// Manager describes how to drive one external package manager.
type Manager struct {
	Name string
	Bin  string

	ListArgs       []string
	OutdatedArgs   []string
	UpgradeArgs    []string
	UpgradeAllArgs []string

	Parser        string
	ListRegex     string
	OutdatedRegex string

	Enabled bool
	Timeout time.Duration
}

// SupportsList reports whether the manager can list installed packages.
func (m Manager) SupportsList() bool { return len(m.ListArgs) > 0 }

// SupportsOutdated reports whether the manager can detect outdated packages.
// uv tool list --outdated and friends; cargo has no built-in equivalent.
func (m Manager) SupportsOutdated() bool { return len(m.OutdatedArgs) > 0 }

// CommandResult is the outcome of one manager command. All three parts matter:
// npm exits non-zero when it finds outdated packages and reports errors on
// stdout, while pipx keeps stdout clean and writes its notes to stderr.
type CommandResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Failure is a manager-level failure, e.g. the manager command failed or its
// output could not be parsed. It is reported per manager so one broken manager
// never hides the others.
type Failure struct {
	Manager string
	Err     error
}

func (f Failure) Error() string {
	return fmt.Sprintf("%s: %v", f.Manager, f.Err)
}

func (f Failure) Unwrap() error { return f.Err }

// UpgradeResult is the outcome of running one manager upgrade command. From and
// To are filled by the caller when it already knows the versions under check.
type UpgradeResult struct {
	Manager string
	Names   []string
	From    string
	To      string
	Output  string
}
