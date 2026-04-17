package config

import "testing"

func TestMergeInstallOptionsUsesGlobalValues(t *testing.T) {
	merged := MergeInstallOptions(
		Section{
			All:          boolPtr(true),
			DownloadOnly: boolPtr(true),
			Source:       boolPtr(true),
			Quiet:        boolPtr(true),
			ShowHash:     boolPtr(true),
			System:       stringPtr("linux/amd64"),
			Target:       stringPtr("~/bin"),
			UpgradeOnly:  boolPtr(true),
		},
		Section{},
		Section{},
		CLIOverrides{},
	)

	if !merged.All || !merged.DownloadOnly || !merged.Source || !merged.Quiet || !merged.ShowHash || !merged.UpgradeOnly {
		t.Fatalf("expected global booleans to be applied, got %#v", merged)
	}
	if merged.System != "linux/amd64" || merged.Target != "~/bin" {
		t.Fatalf("expected global strings to be applied, got %#v", merged)
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
			Quiet: boolPtr(false),
			Tag:   stringPtr("v1.0.0"),
		},
		Section{
			Quiet: boolPtr(true),
			Tag:   stringPtr("v1.1.0"),
		},
		Section{
			Quiet: boolPtr(false),
			Tag:   stringPtr("v1.2.0"),
		},
		CLIOverrides{
			Quiet: boolPtr(true),
			Tag:   stringPtr("v2.0.0"),
		},
	)

	if !merged.Quiet {
		t.Fatalf("expected cli quiet override, got %#v", merged)
	}
	if merged.Tag != "v2.0.0" {
		t.Fatalf("expected cli tag override, got %#v", merged)
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

func boolPtr(v bool) *bool {
	return &v
}

func stringPtr(v string) *string {
	return &v
}
