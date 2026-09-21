package extpkg

// builtinManagers returns the built-in adapters. Command arguments and output
// shapes were measured on Windows; see
// docs/superpowers/specs/2026-09-21-external-managers-design.md section 4.
//
// deno / yarn / go are intentionally not built in.
func builtinManagers() []Manager {
	return []Manager{
		{
			Name:           "npm",
			Bin:            "npm",
			ListArgs:       []string{"ls", "-g", "--depth=0", "--json"},
			OutdatedArgs:   []string{"outdated", "-g", "--json"},
			UpgradeArgs:    []string{"update", "-g"},
			UpgradeAllArgs: []string{"update", "-g"},
			Parser:         ParserNPMJSON,
			Enabled:        true,
		},
		{
			Name:           "pnpm",
			Bin:            "pnpm",
			ListArgs:       []string{"list", "-g", "--depth=0", "--json"},
			OutdatedArgs:   []string{"outdated", "-g", "--json"},
			UpgradeArgs:    []string{"update", "-g"},
			UpgradeAllArgs: []string{"update", "-g"},
			Parser:         ParserPNPMJSON,
			Enabled:        true,
		},
		{
			Name:           "uv",
			Bin:            "uv",
			ListArgs:       []string{"tool", "list"},
			OutdatedArgs:   []string{"tool", "list", "--outdated"},
			UpgradeArgs:    []string{"tool", "upgrade"},
			UpgradeAllArgs: []string{"tool", "upgrade", "--all"},
			Parser:         ParserUVToolText,
			Enabled:        true,
		},
		{
			Name:           "pipx",
			Bin:            "pipx",
			ListArgs:       []string{"list", "--json"},
			OutdatedArgs:   []string{"list", "--json", "--outdated"},
			UpgradeArgs:    []string{"upgrade"},
			UpgradeAllArgs: []string{"upgrade-all"},
			Parser:         ParserPipxJSON,
			Enabled:        true,
		},
		{
			// cargo install --list needs no network, but cargo has no built-in
			// outdated command (that needs the third-party cargo-update).
			Name:        "cargo",
			Bin:         "cargo",
			ListArgs:    []string{"install", "--list"},
			UpgradeArgs: []string{"install"},
			Parser:      ParserCargoText,
			Enabled:     true,
		},
		{
			Name:           "bun",
			Bin:            "bun",
			ListArgs:       []string{"pm", "ls", "-g"},
			OutdatedArgs:   []string{"outdated", "-g"},
			UpgradeArgs:    []string{"update", "-g"},
			UpgradeAllArgs: []string{"update", "-g"},
			Parser:         ParserBunText,
			Enabled:        true,
		},
	}
}
