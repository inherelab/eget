package config

import (
	"strings"

	"github.com/gookit/ext/tomlkit"
	"github.com/inherelab/eget/internal/util/configutil"
)

// SaveMerged writes the configuration back while keeping the untouched parts of
// the existing document: a table whose content did not change keeps its original
// text (comments, key order and formatting included), inside a changed table
// only changed/new keys are re-rendered, tables that left the configuration are
// dropped, and tables that are just defaults are not invented into a file that
// never had them. See tomlkit.Merge for the exact rules.
//
// A missing or unparsable document falls back to a full atomic rewrite.
func SaveMerged(path string, file *File) error {
	rendered, err := dumpManager(encodeConfigFile(file))
	if err != nil {
		return SaveAtomic(path, file)
	}
	// A pristine configuration, rendered by the same serializer, tells the merger
	// which tables only hold defaults.
	defaults, err := dumpManager(encodeConfigFile(NewFile()))
	if err != nil {
		return SaveAtomic(path, file)
	}
	return tomlkit.MergeFile(path, rendered, tomlkit.Options{
		Decode:   decodeDocument,
		Defaults: defaults,
	})
}

// decodeDocument parses TOML through the same engine the loader uses, so
// "unchanged" means the same thing on both sides of the merge.
func decodeDocument(text string) (map[string]any, error) {
	manager, err := configutil.LoadTOMLString("eget-config-merge", text)
	if err != nil {
		return nil, err
	}
	return manager.Data(), nil
}

func dumpManager(manager *configutil.Manager) (string, error) {
	var buf strings.Builder
	if _, err := manager.DumpTo(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
