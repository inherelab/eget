package config

import (
	"testing"

	"github.com/gookit/goutil/testutil/assert"
)

func TestMergeInstallOptionsUsesGlobalValues(t *testing.T) {
	merged := MergeInstallOptions(
		Section{
			ExtractAll:   boolPtr(true),
			DownloadOnly: boolPtr(true),
			Source:       boolPtr(true),
			Quiet:        boolPtr(true),
			ShowHash:     boolPtr(true),
			CacheDir:     stringPtr("~/.cache/eget"),
			ProxyURL:     stringPtr("http://127.0.0.1:7890"),
			GuiTarget:    stringPtr("~/Applications"),
			System:       stringPtr("linux/amd64"),
			Target:       stringPtr("~/bin"),
			UpgradeOnly:  boolPtr(true),
		},
		Section{},
		Section{},
		CLIOverrides{},
	)

	if !merged.ExtractAll || !merged.DownloadOnly || !merged.Source || !merged.Quiet || !merged.ShowHash || !merged.UpgradeOnly {
		t.Fatalf("expected global booleans to be applied, got %#v", merged)
	}
	if merged.System != "linux/amd64" || merged.Target != "~/bin" {
		t.Fatalf("expected global strings to be applied, got %#v", merged)
	}
	if merged.CacheDir != "~/.cache/eget" {
		t.Fatalf("expected global cache dir to be applied, got %#v", merged)
	}
	if merged.ProxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("expected global proxy url to be applied, got %#v", merged)
	}
	if merged.GuiTarget != "~/Applications" {
		t.Fatalf("expected global gui_target to be applied, got %#v", merged)
	}
}

func TestMergeInstallOptionsUsesGUIFromCLIThenPackageThenRepo(t *testing.T) {
	merged := MergeInstallOptions(
		Section{},
		Section{IsGUI: boolPtr(true)},
		Section{},
		CLIOverrides{},
	)
	if !merged.IsGUI {
		t.Fatalf("expected repo is_gui to apply, got %#v", merged)
	}

	merged = MergeInstallOptions(
		Section{},
		Section{IsGUI: boolPtr(false)},
		Section{IsGUI: boolPtr(true)},
		CLIOverrides{},
	)
	if !merged.IsGUI {
		t.Fatalf("expected package is_gui to override repo, got %#v", merged)
	}

	merged = MergeInstallOptions(
		Section{},
		Section{IsGUI: boolPtr(true)},
		Section{IsGUI: boolPtr(true)},
		CLIOverrides{IsGUI: boolPtr(false)},
	)
	if merged.IsGUI {
		t.Fatalf("expected cli is_gui=false to override config, got %#v", merged)
	}
}

func TestMergeInstallOptionsUsesRepoSection(t *testing.T) {
	merged := MergeInstallOptions(
		Section{
			Quiet:  boolPtr(false),
			Target: stringPtr("~/global"),
		},
		Section{
			Quiet:        boolPtr(true),
			CacheDir:     stringPtr("~/repo-cache"),
			Target:       stringPtr("~/repo"),
			AssetFilters: []string{"linux", "amd64"},
			DisableSSL:   boolPtr(true),
		},
		Section{},
		CLIOverrides{},
	)

	if !merged.Quiet {
		t.Fatalf("expected repo quiet override, got %#v", merged)
	}
	if merged.Target != "~/repo" {
		t.Fatalf("expected repo target override, got %#v", merged)
	}
	if merged.CacheDir != "~/repo-cache" {
		t.Fatalf("expected repo cache_dir override, got %#v", merged)
	}
	if len(merged.AssetFilters) != 2 || merged.AssetFilters[0] != "linux" {
		t.Fatalf("expected repo asset filters, got %#v", merged.AssetFilters)
	}
	if !merged.DisableSSL {
		t.Fatalf("expected repo disable_ssl override, got %#v", merged)
	}
}

func TestMergeInstallOptionsCLIOverridesRepoAndGlobal(t *testing.T) {
	merged := MergeInstallOptions(
		Section{
			Quiet:    boolPtr(false),
			CacheDir: stringPtr("~/global-cache"),
			Tag:      stringPtr("v1.0.0"),
		},
		Section{
			Quiet:    boolPtr(true),
			CacheDir: stringPtr("~/repo-cache"),
			Tag:      stringPtr("v1.1.0"),
		},
		Section{
			Quiet:    boolPtr(false),
			CacheDir: stringPtr("~/pkg-cache"),
			Tag:      stringPtr("v1.2.0"),
		},
		CLIOverrides{
			Quiet:    boolPtr(true),
			CacheDir: stringPtr("~/cli-cache"),
			Tag:      stringPtr("v2.0.0"),
		},
	)

	if !merged.Quiet {
		t.Fatalf("expected cli quiet override, got %#v", merged)
	}
	if merged.Tag != "v2.0.0" {
		t.Fatalf("expected cli tag override, got %#v", merged)
	}
	if merged.CacheDir != "~/cli-cache" {
		t.Fatalf("expected cli cache_dir override, got %#v", merged)
	}
}

func TestMergeInstallOptionsPackageOverridesRepoAndGlobal(t *testing.T) {
	merged := MergeInstallOptions(
		Section{
			Target: stringPtr("~/global"),
		},
		Section{
			Target: stringPtr("~/repo"),
		},
		Section{
			Target: stringPtr("~/package"),
		},
		CLIOverrides{},
	)

	if merged.Target != "~/package" {
		t.Fatalf("expected package target override, got %#v", merged)
	}
}

func TestMergeInstallOptionsMergesSourcePath(t *testing.T) {
	merged := MergeInstallOptions(
		Section{SourcePath: stringPtr("global")},
		Section{SourcePath: stringPtr("repo")},
		Section{SourcePath: stringPtr("package")},
		CLIOverrides{SourcePath: stringPtr("cli")},
	)

	assert.Eq(t, "cli", merged.SourcePath)

	merged = MergeInstallOptions(
		Section{SourcePath: stringPtr("global")},
		Section{SourcePath: stringPtr("repo")},
		Section{SourcePath: stringPtr("package")},
		CLIOverrides{},
	)

	assert.Eq(t, "package", merged.SourcePath)

	merged = MergeInstallOptions(
		Section{SourcePath: stringPtr("global")},
		Section{SourcePath: stringPtr("repo")},
		Section{},
		CLIOverrides{},
	)

	assert.Eq(t, "repo", merged.SourcePath)
}

func boolPtr(v bool) *bool {
	return &v
}

func stringPtr(v string) *string {
	return &v
}
