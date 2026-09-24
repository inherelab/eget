package config

import (
	"os"
	"reflect"
	"strings"

	"github.com/inherelab/eget/internal/util/atomicfile"
	"github.com/inherelab/eget/internal/util/configutil"
)

// tomlBlock is one textual chunk of a TOML document: the keys before the first
// table header (name "") or one table with everything that follows it,
// comments included.
type tomlBlock struct {
	name string
	text string
}

// SaveMerged writes the configuration back while keeping untouched parts of the
// existing document byte-for-byte, so comments, key order and formatting
// survive an edit:
//
//   - a table whose parsed content did not change is kept as-is;
//   - inside a changed table, lines of keys that did not change (and the
//     comments around them) are kept, only changed/new keys are re-rendered;
//   - tables that left the configuration are dropped.
//
// The merged document is parsed again before writing: if it would not round-trip
// (or the file is missing/unparsable) it falls back to a full atomic write.
func SaveMerged(path string, file *File) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SaveAtomic(path, file)
		}
		return err
	}

	merged, err := mergeTOMLText(string(existing), file)
	if err != nil {
		return SaveAtomic(path, file)
	}
	if _, err := configutil.LoadTOMLString("eget-config-merge-check", merged); err != nil {
		return SaveAtomic(path, file)
	}
	return atomicfile.WriteFile(path, []byte(merged), 0o644)
}

func mergeTOMLText(text string, file *File) (string, error) {
	oldManager, err := configutil.LoadTOMLString("eget-config-merge", text)
	if err != nil {
		return "", err
	}
	rendered := encodeConfigFile(file)
	renderedText, err := dumpManager(rendered)
	if err != nil {
		return "", err
	}
	// A pristine configuration tells us which tables consist of defaults only.
	defaultData := encodeConfigFile(NewFile()).Data()

	oldData := oldManager.Data()
	newData := rendered.Data()

	renderedBlocks := splitTOMLBlocks(renderedText)
	renderedByName := make(map[string]string, len(renderedBlocks))
	for _, block := range renderedBlocks {
		renderedByName[block.name] = block.text
	}

	var out strings.Builder
	kept := make(map[string]bool, len(renderedBlocks))
	for _, block := range splitTOMLBlocks(text) {
		next, ok := renderedByName[block.name]
		if !ok {
			// Not a table the configuration still has: the header block (keys and
			// comments before the first [table]) has no counterpart, so keep it;
			// a removed table is dropped whole.
			if block.name == "" {
				out.WriteString(block.text)
			}
			continue
		}
		kept[block.name] = true
		if reflect.DeepEqual(lookupTablePath(oldData, block.name), lookupTablePath(newData, block.name)) {
			out.WriteString(block.text)
			continue
		}
		out.WriteString(mergeChangedTable(block.text, next))
	}
	// Tables the rendered document adds: only when their content differs from a
	// pristine configuration, so default-only tables (empty sections, default
	// api_cache/ghproxy values) are not invented into a file that never had them.
	for _, block := range renderedBlocks {
		if kept[block.name] {
			continue
		}
		// A table that only has a header (its content lives in sub-tables such as
		// [packages.fd]) is not worth writing on its own.
		if !tableHasKeys(block.text) {
			continue
		}
		if reflect.DeepEqual(lookupTablePath(defaultData, block.name), lookupTablePath(newData, block.name)) {
			continue
		}
		out.WriteString(block.text)
	}
	return out.String(), nil
}

// tableHasKeys reports whether a rendered block holds any key line below its
// header.
func tableHasKeys(text string) bool {
	headerSeen := false
	for _, raw := range strings.SplitAfter(text, "\n") {
		trimmed := strings.TrimSpace(raw)
		if !headerSeen && strings.HasPrefix(trimmed, "[") {
			headerSeen = true
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		return true
	}
	return false
}

// mergeChangedTable keeps the original lines of keys whose value stayed the same
// (and the comments around them), taking changed and new keys from the rendered
// table. A key that only exists in the old table was removed and is dropped.
// Anything it cannot analyse line-by-line falls back to the rendered text.
func mergeChangedTable(oldText, newText string) string {
	oldTable, okOld := parseSimpleKeyLines(oldText)
	newTable, okNew := parseSimpleKeyLines(newText)
	if !okOld || !okNew {
		return newText
	}

	var out strings.Builder
	out.WriteString(oldTable.header)
	for _, key := range newTable.order {
		line := newTable.keys[key]
		old, existed := oldTable.keys[key]
		switch {
		case existed && simpleKeyValue(old.line) == simpleKeyValue(line.line):
			// Untouched key: original text, comment and all.
			out.WriteString(old.leading)
			out.WriteString(old.line)
		case existed:
			// The value changed: keep the comments that belonged to the key and
			// only swap the value line in.
			out.WriteString(old.leading)
			out.WriteString(withInlineComment(line.line, old.line))
		default:
			// New key.
			out.WriteString(line.leading)
			out.WriteString(line.line)
		}
	}
	return out.String()
}

// simpleKeyValue returns the value part of a "key = value" line without
// indentation or a trailing comment, so purely cosmetic differences (the
// renderer indents, a hand-written file may not) do not read as a change.
func simpleKeyValue(line string) string {
	trimmed := strings.TrimSpace(line)
	index := strings.Index(trimmed, "=")
	if index <= 0 {
		return trimmed
	}
	value := strings.TrimSpace(trimmed[index+1:])
	if comment, ok := inlineComment(value); ok {
		value = strings.TrimSpace(strings.TrimSuffix(value, comment))
	}
	return value
}

// withInlineComment carries a " # note" suffix over from the previous version of
// the line, if there was one.
func withInlineComment(newLine, oldLine string) string {
	comment, ok := inlineComment(oldLine)
	if !ok {
		return newLine
	}
	body := strings.TrimRight(newLine, "\r\n")
	suffix := "\n"
	if strings.HasSuffix(newLine, "\r\n") {
		suffix = "\r\n"
	}
	return body + " " + comment + suffix
}

// inlineComment returns the trailing comment of a line, ignoring '#' inside
// quoted values.
func inlineComment(line string) (string, bool) {
	inString := false
	var quote byte
	for index := 0; index < len(line); index++ {
		char := line[index]
		if inString {
			if char == quote {
				inString = false
			}
			continue
		}
		switch char {
		case '"', '\'':
			inString = true
			quote = char
		case '#':
			return strings.TrimSpace(line[index:]), true
		}
	}
	return "", false
}

type simpleKeyLine struct {
	leading string
	line    string
}

type simpleTable struct {
	header string
	order  []string
	keys   map[string]simpleKeyLine
}

// parseSimpleKeyLines splits a table into its header line and one entry per
// "key = value" line. It reports false when a line cannot be handled that way
// (multi-line values, sub-tables), because the caller must then avoid merging.
func parseSimpleKeyLines(text string) (simpleTable, bool) {
	table := simpleTable{keys: map[string]simpleKeyLine{}}
	var leading strings.Builder
	headerSeen := false

	for _, raw := range strings.SplitAfter(text, "\n") {
		trimmed := strings.TrimSpace(raw)
		switch {
		case !headerSeen && strings.HasPrefix(trimmed, "["):
			table.header += raw
			headerSeen = true
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
			leading.WriteString(raw)
		default:
			key, ok := simpleKeyName(trimmed)
			if !ok {
				return simpleTable{}, false
			}
			table.order = append(table.order, key)
			table.keys[key] = simpleKeyLine{leading: leading.String(), line: raw}
			leading.Reset()
		}
	}
	return table, true
}

func simpleKeyName(line string) (string, bool) {
	index := strings.Index(line, "=")
	if index <= 0 {
		return "", false
	}
	key := strings.TrimSpace(line[:index])
	if key == "" || strings.ContainsAny(key, " \t\"'") {
		return "", false
	}
	if unbalancedValue(strings.TrimSpace(line[index+1:])) {
		return "", false
	}
	return key, true
}

// unbalancedValue reports a value that looks like it continues on another line
// (an unterminated array, inline table or string).
func unbalancedValue(value string) bool {
	depth := 0
	inString := false
	var quote byte
	for index := 0; index < len(value); index++ {
		char := value[index]
		if inString {
			if char == quote {
				inString = false
			}
			continue
		}
		switch char {
		case '"', '\'':
			inString = true
			quote = char
		case '[', '{':
			depth++
		case ']', '}':
			depth--
		}
	}
	return inString || depth != 0
}

func dumpManager(manager *configutil.Manager) (string, error) {
	var buf strings.Builder
	if _, err := manager.DumpTo(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// splitTOMLBlocks splits a document at table headers so each block can be kept
// or replaced as a unit.
func splitTOMLBlocks(text string) []tomlBlock {
	blocks := []tomlBlock{}
	var current strings.Builder
	name := ""
	flush := func() {
		if current.Len() > 0 {
			blocks = append(blocks, tomlBlock{name: name, text: current.String()})
			current.Reset()
		}
	}

	for _, line := range strings.SplitAfter(text, "\n") {
		if header, ok := tomlHeaderName(strings.TrimSpace(line)); ok {
			flush()
			name = header
		}
		current.WriteString(line)
	}
	flush()
	return blocks
}

func tomlHeaderName(line string) (string, bool) {
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return "", false
	}
	// Arrays of tables are not used by eget's configuration.
	if strings.HasPrefix(line, "[[") {
		return "", false
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
	if inner == "" {
		return "", false
	}
	return inner, true
}

// lookupTablePath walks a dotted table path ("packages.fd") through decoded data.
func lookupTablePath(data map[string]any, path string) any {
	if path == "" {
		return nil
	}
	var current any = data
	for _, part := range strings.Split(path, ".") {
		node, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = node[part]
		if !ok {
			return nil
		}
	}
	return current
}
