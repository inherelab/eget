package extpkg

import (
	"sort"
	"time"

	cfgpkg "github.com/inherelab/eget/internal/config"
)

// Managers merges the built-in adapters with the [ext.<name>] config
// sections:
//
//   - a section named after a builtin overrides that builtin
//   - enabled = false removes the manager (builtin or configured)
//   - an unknown name adds a new manager
//
// The result is sorted by name and always carries a usable Bin, Parser and
// Timeout.
func Managers(cfg *cfgpkg.File) []Manager {
	byName := make(map[string]Manager)
	for _, manager := range builtinManagers() {
		byName[manager.Name] = manager
	}

	if cfg != nil {
		for _, name := range sortedManagerSectionNames(cfg) {
			section := cfg.Ext[name]
			if section.Enabled != nil && !*section.Enabled {
				delete(byName, name)
				continue
			}
			base, ok := byName[name]
			if !ok {
				base = Manager{Name: name, Bin: name, Enabled: true}
			}
			byName[name] = mergeManagerSection(base, section)
		}
	}

	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)

	managers := make([]Manager, 0, len(names))
	for _, name := range names {
		managers = append(managers, normalizeManager(byName[name]))
	}
	return managers
}

func sortedManagerSectionNames(cfg *cfgpkg.File) []string {
	names := make([]string, 0, len(cfg.Ext))
	for name := range cfg.Ext {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func normalizeManager(manager Manager) Manager {
	if manager.Bin == "" {
		manager.Bin = manager.Name
	}
	if manager.Parser == "" {
		manager.Parser = ParserLinesRegex
	}
	if manager.Timeout <= 0 {
		manager.Timeout = DefaultTimeout
	}
	return manager
}

func mergeManagerSection(base Manager, section cfgpkg.ManagerSection) Manager {
	merged := base
	if section.Bin != nil && *section.Bin != "" {
		merged.Bin = *section.Bin
	}
	// nil means "not configured" and keeps the builtin value; an explicit empty
	// list clears the command (for example to disable outdated detection).
	if section.ListArgs != nil {
		merged.ListArgs = cloneArgs(section.ListArgs)
	}
	if section.OutdatedArgs != nil {
		merged.OutdatedArgs = cloneArgs(section.OutdatedArgs)
	}
	if section.UpgradeArgs != nil {
		merged.UpgradeArgs = cloneArgs(section.UpgradeArgs)
	}
	if section.UpgradeAllArgs != nil {
		merged.UpgradeAllArgs = cloneArgs(section.UpgradeAllArgs)
	}
	if section.Parser != nil && *section.Parser != "" {
		merged.Parser = *section.Parser
	}
	if section.ListRegex != nil {
		merged.ListRegex = *section.ListRegex
	}
	if section.OutdatedRegex != nil {
		merged.OutdatedRegex = *section.OutdatedRegex
	}
	if section.Enabled != nil {
		merged.Enabled = *section.Enabled
	}
	if section.Timeout != nil && *section.Timeout > 0 {
		merged.Timeout = time.Duration(*section.Timeout) * time.Second
	}
	return merged
}

func cloneArgs(args []string) []string {
	if len(args) == 0 {
		return []string{}
	}
	return append([]string(nil), args...)
}
