package extpkg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ParseList parses the output of a manager's list command into installed
// packages. An empty slice is a valid result: no packages are installed.
func ParseList(manager Manager, res CommandResult) ([]Package, error) {
	switch manager.Parser {
	case ParserNPMJSON:
		return parseJSONPackages(manager, res, false)
	case ParserPNPMJSON:
		return parseJSONPackages(manager, res, true)
	case ParserPipxJSON:
		return parsePipxJSON(manager, res)
	case ParserUVToolText:
		return parseUVToolList(manager, res)
	case ParserCargoText:
		return parseCargoList(manager, res)
	case ParserBunText:
		return parseBunList(manager, res)
	case ParserScoopTable:
		return parseScoopTable(manager, res, false)
	case ParserLinesRegex:
		return parseLinesRegex(manager, res, manager.ListRegex, false)
	default:
		return nil, fmt.Errorf("unknown parser %q", manager.Parser)
	}
}

// ParseOutdated parses the output of a manager's outdated command, which only
// reports the packages that are behind.
func ParseOutdated(manager Manager, res CommandResult) ([]Package, error) {
	switch manager.Parser {
	case ParserNPMJSON:
		return parseJSONPackages(manager, res, false)
	case ParserPNPMJSON:
		return parseJSONPackages(manager, res, true)
	case ParserPipxJSON:
		return parsePipxOutdated(manager, res)
	case ParserUVToolText:
		return parseUVToolList(manager, res)
	case ParserBunText:
		return parseBunOutdated(manager, res)
	case ParserScoopTable:
		return parseScoopTable(manager, res, true)
	case ParserCargoText, ParserLinesRegex:
		return parseLinesRegex(manager, res, manager.OutdatedRegex, true)
	default:
		return nil, fmt.Errorf("unknown parser %q", manager.Parser)
	}
}

// npm ls -g:   {"dependencies":{"<name>":{"version":"x"}}}
// npm outdated: {"<name>":{"current":"x","wanted":"y","latest":"z"}}
// pnpm list -g: [{"dependencies":{"<name>":{"version":"x"}}}]  (top-level array)
func parseJSONPackages(manager Manager, res CommandResult, topLevelArray bool) ([]Package, error) {
	if strings.TrimSpace(string(res.Stdout)) == "" {
		return nil, outputError(manager, res, "no output")
	}

	var root map[string]any
	if topLevelArray {
		// pnpm list -g --json is a top-level array, but pnpm outdated -g --json
		// is a plain object like npm's, so fall through when it is not an array.
		var items []map[string]any
		if err := json.Unmarshal(res.Stdout, &items); err == nil {
			for _, item := range items {
				if deps, ok := asMap(item["dependencies"]); ok {
					return packagesFromDependencies(manager, deps), nil
				}
			}
			return nil, nil
		}
	}

	if err := json.Unmarshal(res.Stdout, &root); err != nil {
		return nil, outputError(manager, res, err.Error())
	}
	if deps, ok := asMap(root["dependencies"]); ok {
		return packagesFromDependencies(manager, deps), nil
	}

	// Otherwise this is an outdated map: name -> {current, latest, ...}.
	packages := make([]Package, 0, len(root))
	for name, value := range root {
		fields, ok := asMap(value)
		if !ok {
			continue
		}
		packages = append(packages, Package{
			Manager: manager.Name,
			Name:    name,
			Version: stringField(fields, "current"),
			Latest:  stringField(fields, "latest"),
		})
	}
	return sortPackages(packages), nil
}

func packagesFromDependencies(manager Manager, deps map[string]any) []Package {
	packages := make([]Package, 0, len(deps))
	for name, value := range deps {
		fields, ok := asMap(value)
		if !ok {
			continue
		}
		packages = append(packages, Package{
			Manager: manager.Name,
			Name:    name,
			Version: stringField(fields, "version"),
		})
	}
	return sortPackages(packages)
}

// pipx list --json: {"venvs":{"<name>":{"metadata":{"main_package":{
// "package":"x","package_version":"1.0"}}}}}
//
// pipx list --json --outdated uses a different schema *and* different field
// names: {"data":{"packages":[{"package":"x","version":"1.0",
// "latest_version":"2.0"}]},"status":"success","errors":[]}
func parsePipxJSON(manager Manager, res CommandResult) ([]Package, error) {
	if strings.TrimSpace(string(res.Stdout)) == "" {
		return nil, outputError(manager, res, "no output")
	}

	var body struct {
		Venvs map[string]struct {
			Metadata struct {
				MainPackage struct {
					Package        string `json:"package"`
					PackageVersion string `json:"package_version"`
				} `json:"main_package"`
			} `json:"metadata"`
		} `json:"venvs"`
	}
	if err := json.Unmarshal(res.Stdout, &body); err != nil {
		return nil, outputError(manager, res, err.Error())
	}

	packages := make([]Package, 0, len(body.Venvs))
	for name, venv := range body.Venvs {
		main := venv.Metadata.MainPackage
		if main.Package != "" {
			name = main.Package
		}
		packages = append(packages, Package{
			Manager: manager.Name,
			Name:    name,
			Version: main.PackageVersion,
		})
	}
	return sortPackages(packages), nil
}

func parsePipxOutdated(manager Manager, res CommandResult) ([]Package, error) {
	if strings.TrimSpace(string(res.Stdout)) == "" {
		return nil, outputError(manager, res, "no output")
	}

	var body struct {
		Data struct {
			Packages []struct {
				Package       string `json:"package"`
				Version       string `json:"version"`
				LatestVersion string `json:"latest_version"`
			} `json:"packages"`
		} `json:"data"`
		Errors []string `json:"errors"`
		Status string   `json:"status"`
	}
	if err := json.Unmarshal(res.Stdout, &body); err != nil {
		return nil, outputError(manager, res, err.Error())
	}
	// The envelope reports failures inside the payload, not only via exit code.
	if len(body.Errors) > 0 || (body.Status != "" && body.Status != "success") {
		return nil, outputError(manager, res, strings.Join(body.Errors, "; "))
	}

	packages := make([]Package, 0, len(body.Data.Packages))
	for _, item := range body.Data.Packages {
		packages = append(packages, Package{
			Manager: manager.Name,
			Name:    item.Package,
			Version: item.Version,
			Latest:  item.LatestVersion,
		})
	}
	return sortPackages(packages), nil
}

// uv tool list: "name v1.2.3" followed by indented "- executable" lines.
// uv tool list --outdated adds " [latest: 1.2.4]".
var uvToolLine = regexp.MustCompile(`^(\S+)\s+v?([^\s\[]+)(?:\s+\[latest:\s*([^\]]+)\])?$`)

func parseUVToolList(manager Manager, res CommandResult) ([]Package, error) {
	packages := make([]Package, 0)
	for _, line := range outputLines(res.Stdout) {
		trimmed := strings.TrimSpace(line)
		// Skip blank lines and the executable entries. uv prints those as
		// "- <executable>" (not always indented), and they are not tools.
		if trimmed == "" || strings.HasPrefix(trimmed, "-") {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		matches := uvToolLine.FindStringSubmatch(trimmed)
		if matches == nil {
			continue
		}
		packages = append(packages, Package{
			Manager: manager.Name,
			Name:    matches[1],
			Version: matches[2],
			Latest:  strings.TrimSpace(matches[3]),
		})
	}
	if len(packages) == 0 && res.ExitCode != 0 {
		return nil, outputError(manager, res, "no packages parsed")
	}
	return sortPackages(packages), nil
}

// cargo install --list prints "<name> v<version> (<source>):" followed by
// indented executable names. With nothing installed the output is empty.
var cargoInstallLine = regexp.MustCompile(`^(\S+)\s+v([^\s:]+)\s*(?:\(([^)]*)\))?:\s*$`)

func parseCargoList(manager Manager, res CommandResult) ([]Package, error) {
	packages := make([]Package, 0)
	for _, line := range outputLines(res.Stdout) {
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		matches := cargoInstallLine.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		packages = append(packages, Package{Manager: manager.Name, Name: matches[1], Version: matches[2]})
	}
	if len(packages) == 0 && res.ExitCode != 0 {
		return nil, outputError(manager, res, "no packages parsed")
	}
	return sortPackages(packages), nil
}

// bun pm ls -g prints a root line then tree lines:
//
//	<dir> node_modules (1 installed)
//	└── is-number@1.0.0
func parseBunList(manager Manager, res CommandResult) ([]Package, error) {
	packages := make([]Package, 0)
	for _, line := range outputLines(res.Stdout) {
		idx := strings.Index(line, treeBranch)
		if idx < 0 {
			continue
		}
		name, version := splitNameVersion(strings.TrimSpace(line[idx+len(treeBranch):]))
		if name == "" {
			continue
		}
		packages = append(packages, Package{Manager: manager.Name, Name: name, Version: version})
	}
	if len(packages) == 0 && res.ExitCode != 0 && !bunHasNoPackages(res) {
		return nil, outputError(manager, res, "no packages parsed")
	}
	return sortPackages(packages), nil
}

// bun outdated -g prints a banner and a markdown-ish table:
//
//	bun outdated v1.4.2 (744846f84)
//	| Package   | Current | Update | Latest |
//	|-----------|---------|--------|--------|
//	| is-number | 1.0.0   | 1.0.0  | 7.0.0  |
func parseBunOutdated(manager Manager, res CommandResult) ([]Package, error) {
	headerSeen := false
	packages := make([]Package, 0)
	for _, line := range outputLines(res.Stdout) {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := tableCells(trimmed)
		if len(cells) < 4 {
			continue
		}
		if strings.EqualFold(cells[0], "package") {
			headerSeen = true
			continue
		}
		if isTableSeparator(cells[0]) {
			continue
		}
		packages = append(packages, Package{
			Manager: manager.Name,
			Name:    cells[0],
			Version: cells[1],
			Latest:  cells[3],
		})
	}
	if !headerSeen && res.ExitCode != 0 && !bunHasNoPackages(res) {
		return nil, outputError(manager, res, "no outdated table found")
	}
	return sortPackages(packages), nil
}

const treeBranch = "── "

// bunHasNoPackages reports the "no global packages yet" case, which bun treats
// as an error. The two commands word it differently:
//
//	No package.json was found for directory "..."
//	error: missing package.json, nothing outdated
func bunHasNoPackages(res CommandResult) bool {
	text := strings.ToLower(string(res.Stderr))
	for _, marker := range []string{"no package.json", "missing package.json", "missingpackagejson"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// scoop list:   "Name / Version / Source / Updated / Info" aligned columns.
// scoop status: "Name / Installed Version / Latest Version / ..." columns with
// one row per outdated package. Both print a table header, a dashes separator
// and notice lines like "Installed apps:" or "WARN ...".
// ansiPattern matches SGR color escapes. PowerShell's Format-Table colors the
// header and separator cells of scoop's output, which would otherwise defeat
// the row classification below.
var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

func parseScoopTable(manager Manager, res CommandResult, outdated bool) ([]Package, error) {
	packages := []Package{}
	for _, line := range outputLines(res.Stdout) {
		line = ansiPattern.ReplaceAllString(line, "")
		fields := strings.Fields(line)
		if !isScoopDataRow(line, fields) {
			continue
		}
		if outdated && len(fields) < 3 {
			continue
		}
		pkg := Package{Manager: manager.Name, Name: fields[0], Version: fields[1]}
		if outdated {
			pkg.Latest = fields[2]
		}
		packages = append(packages, pkg)
	}
	return sortPackages(packages), nil
}

// isScoopDataRow keeps only the table rows of "scoop list" / "scoop status":
// it drops the "Name ..." header, its dashes separator and notice lines like
// "Installed apps:" or "WARN Scoop bucket(s) out of date.".
func isScoopDataRow(line string, fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	switch fields[0] {
	case "Name", "WARN", "WARNING", "INFO", "ERROR":
		return false
	}
	if strings.Trim(fields[0], "-") == "" || strings.HasSuffix(strings.TrimSpace(line), ":") {
		return false
	}
	return true
}

// parseLinesRegex parses one package per line using named groups: name,
// version, and an optional latest.
func parseLinesRegex(manager Manager, res CommandResult, expr string, outdated bool) ([]Package, error) {
	if strings.TrimSpace(expr) == "" {
		return nil, fmt.Errorf("%s: parser %s requires a regex", manager.Name, ParserLinesRegex)
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid regex: %w", manager.Name, err)
	}

	indexes := make(map[string]int)
	for i, name := range re.SubexpNames() {
		if name != "" {
			indexes[name] = i
		}
	}
	if _, ok := indexes["name"]; !ok {
		return nil, fmt.Errorf("%s: regex must define a (?P<name>...) group", manager.Name)
	}

	packages := make([]Package, 0)
	for _, line := range outputLines(res.Stdout) {
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		pkg := Package{Manager: manager.Name, Name: matches[indexes["name"]]}
		if i, ok := indexes["version"]; ok {
			pkg.Version = matches[i]
		}
		if i, ok := indexes["latest"]; ok {
			pkg.Latest = matches[i]
		}
		packages = append(packages, pkg)
	}
	if outdated && len(packages) == 0 && res.ExitCode != 0 {
		return nil, outputError(manager, res, "no packages parsed")
	}
	return sortPackages(packages), nil
}

// outputError reports a manager failure with both streams attached: npm writes
// errors to stdout, pipx and cargo write them to stderr, and either may hold the
// only clue about what went wrong.
func outputError(manager Manager, res CommandResult, reason string) error {
	detail := strings.TrimSpace(string(res.Stdout))
	if stderr := strings.TrimSpace(string(res.Stderr)); stderr != "" {
		if detail == "" {
			detail = stderr
		} else {
			detail += "; " + stderr
		}
	}
	if detail == "" {
		return fmt.Errorf("%s: %s", manager.Name, reason)
	}
	return fmt.Errorf("%s: %s (exit %d): %s", manager.Name, reason, res.ExitCode, detail)
}

func outputLines(data []byte) []string {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	return strings.Split(text, "\n")
}

func asMap(value any) (map[string]any, bool) {
	fields, ok := value.(map[string]any)
	return fields, ok
}

func stringField(fields map[string]any, key string) string {
	text, _ := fields[key].(string)
	return text
}

// splitNameVersion splits "is-number@1.0.0" and "@scope/pkg@1.0.0".
func splitNameVersion(text string) (string, string) {
	idx := strings.LastIndex(text, "@")
	if idx <= 0 {
		return text, ""
	}
	return text[:idx], text[idx+1:]
}

func tableCells(line string) []string {
	parts := strings.Split(strings.Trim(line, "|"), "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

func isTableSeparator(cell string) bool {
	return cell != "" && strings.Trim(cell, "-") == ""
}

func sortPackages(packages []Package) []Package {
	sort.Slice(packages, func(i, j int) bool { return packages[i].Name < packages[j].Name })
	return packages
}
