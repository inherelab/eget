package config

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
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
			UserAgent:    stringPtr("global-agent"),
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
	if merged.UserAgent != "global-agent" {
		t.Fatalf("expected global user_agent to be applied, got %#v", merged)
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

func TestMergeInstallOptionsUsesPrereleaseFromCLIThenPackageThenRepo(t *testing.T) {
	merged := MergeInstallOptions(
		Section{},
		Section{Prerelease: boolPtr(true)},
		Section{},
		CLIOverrides{},
	)
	assert.True(t, merged.Prerelease)

	merged = MergeInstallOptions(
		Section{},
		Section{Prerelease: boolPtr(false)},
		Section{Prerelease: boolPtr(true)},
		CLIOverrides{},
	)
	assert.True(t, merged.Prerelease)

	merged = MergeInstallOptions(
		Section{},
		Section{Prerelease: boolPtr(true)},
		Section{Prerelease: boolPtr(true)},
		CLIOverrides{Prerelease: boolPtr(false)},
	)
	assert.False(t, merged.Prerelease)
}

func TestMergeInstallOptionsUsesInstallModeFromPackageThenRepo(t *testing.T) {
	merged := MergeInstallOptions(
		Section{},
		Section{InstallMode: stringPtr("portable")},
		Section{InstallMode: stringPtr("installer")},
		CLIOverrides{},
	)
	assert.Eq(t, "installer", merged.InstallMode)

	merged = MergeInstallOptions(
		Section{},
		Section{InstallMode: stringPtr("installer")},
		Section{},
		CLIOverrides{},
	)
	assert.Eq(t, "installer", merged.InstallMode)
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

func TestMergeInstallOptionsChunkConcurrencyPrecedence(t *testing.T) {
	globalChunk := 1
	repoChunk := 2
	pkgChunk := 3
	cliChunk := 4

	merged := MergeInstallOptions(
		Section{ChunkConcurrency: &globalChunk},
		Section{ChunkConcurrency: &repoChunk},
		Section{ChunkConcurrency: &pkgChunk},
		CLIOverrides{ChunkConcurrency: &cliChunk},
	)
	assert.Eq(t, 4, merged.ChunkConcurrency)

	merged = MergeInstallOptions(
		Section{ChunkConcurrency: &globalChunk},
		Section{ChunkConcurrency: &repoChunk},
		Section{ChunkConcurrency: &pkgChunk},
		CLIOverrides{},
	)
	assert.Eq(t, 3, merged.ChunkConcurrency)

	merged = MergeInstallOptions(
		Section{ChunkConcurrency: &globalChunk},
		Section{ChunkConcurrency: &repoChunk},
		Section{},
		CLIOverrides{},
	)
	assert.Eq(t, 2, merged.ChunkConcurrency)
}

func TestMergeInstallOptionsStripComponentsPrecedence(t *testing.T) {
	globalStrip := 1
	repoStrip := 2
	pkgStrip := 3
	cliStrip := 4

	merged := MergeInstallOptions(
		Section{StripComponents: &globalStrip},
		Section{StripComponents: &repoStrip},
		Section{StripComponents: &pkgStrip},
		CLIOverrides{StripComponents: &cliStrip},
	)
	assert.Eq(t, 4, merged.StripComponents)

	merged = MergeInstallOptions(
		Section{StripComponents: &globalStrip},
		Section{StripComponents: &repoStrip},
		Section{StripComponents: &pkgStrip},
		CLIOverrides{},
	)
	assert.Eq(t, 3, merged.StripComponents)

	merged = MergeInstallOptions(
		Section{StripComponents: &globalStrip},
		Section{StripComponents: &repoStrip},
		Section{},
		CLIOverrides{},
	)
	assert.Eq(t, 2, merged.StripComponents)
}

func TestMergeInstallOptionsMergesSys7zPath(t *testing.T) {
	globalPath := "C:/global/7z.exe"
	repoPath := "C:/repo/7z.exe"
	pkgPath := "C:/pkg/7z.exe"

	merged := MergeInstallOptions(
		Section{Sys7zPath: &globalPath},
		Section{},
		Section{},
		CLIOverrides{},
	)
	assert.Eq(t, globalPath, merged.Sys7zPath)

	merged = MergeInstallOptions(
		Section{Sys7zPath: &globalPath},
		Section{Sys7zPath: &repoPath},
		Section{},
		CLIOverrides{},
	)
	assert.Eq(t, repoPath, merged.Sys7zPath)

	merged = MergeInstallOptions(
		Section{Sys7zPath: &globalPath},
		Section{Sys7zPath: &repoPath},
		Section{Sys7zPath: &pkgPath},
		CLIOverrides{},
	)
	assert.Eq(t, pkgPath, merged.Sys7zPath)
}

func TestMergeInstallOptionsMergesRenameFiles(t *testing.T) {
	merged := MergeInstallOptions(
		Section{RenameFiles: map[string]string{"global.exe": "global-renamed.exe"}},
		Section{RenameFiles: map[string]string{"repo.exe": "repo-renamed.exe"}},
		Section{RenameFiles: map[string]string{"pkg.exe": "pkg-renamed.exe"}},
		CLIOverrides{},
	)
	assert.Eq(t, map[string]string{"pkg.exe": "pkg-renamed.exe"}, merged.RenameFiles)

	merged = MergeInstallOptions(
		Section{RenameFiles: map[string]string{"global.exe": "global-renamed.exe"}},
		Section{RenameFiles: map[string]string{"repo.exe": "repo-renamed.exe"}},
		Section{},
		CLIOverrides{},
	)
	assert.Eq(t, map[string]string{"repo.exe": "repo-renamed.exe"}, merged.RenameFiles)

	merged = MergeInstallOptions(
		Section{RenameFiles: map[string]string{"global.exe": "global-renamed.exe"}},
		Section{RenameFiles: map[string]string{"repo.exe": "repo-renamed.exe"}},
		Section{RenameFiles: map[string]string{"pkg.exe": "pkg-renamed.exe"}},
		CLIOverrides{RenameFiles: &map[string]string{"cli.exe": "cli-renamed.exe"}},
	)
	assert.Eq(t, map[string]string{"cli.exe": "cli-renamed.exe"}, merged.RenameFiles)
}

func TestMergeInstallOptionsUsesSelectFirstFromCLIThenPackageThenRepo(t *testing.T) {
	merged := MergeInstallOptions(
		Section{SelectFirst: boolPtr(true)},
		Section{SelectFirst: boolPtr(false)},
		Section{},
		CLIOverrides{},
	)
	assert.False(t, merged.SelectFirst)

	merged = MergeInstallOptions(
		Section{SelectFirst: boolPtr(false)},
		Section{SelectFirst: boolPtr(false)},
		Section{SelectFirst: boolPtr(true)},
		CLIOverrides{SelectFirst: boolPtr(true)},
	)
	assert.True(t, merged.SelectFirst)

	merged = MergeInstallOptions(
		Section{}, Section{}, Section{}, CLIOverrides{SelectFirst: boolPtr(true)},
	)
	assert.True(t, merged.SelectFirst)
}

func TestMergeInstallOptionsMergesURLTemplateFields(t *testing.T) {
	merged := MergeInstallOptions(
		Section{
			URLTemplate: stringPtr("global"),
			OSMap:       map[string]string{"windows": "global-win"},
		},
		Section{
			URLTemplate: stringPtr("repo"),
			OSMap:       map[string]string{"windows": "repo-win"},
		},
		Section{
			URLTemplate:   stringPtr("package"),
			LatestURL:     stringPtr("https://example.com/latest"),
			LatestFormat:  stringPtr("text"),
			OSMap:         map[string]string{"windows": "win32"},
			ArchMap:       map[string]string{"amd64": "x64"},
			ExtMap:        map[string]string{"windows": ".exe"},
			LibcMap:       map[string]string{"musl": "-musl"},
			InstallAction: stringPtr("run-asset"),
			InstallArgs:   []string{"install", "latest"},
		},
		CLIOverrides{},
	)

	assert.Eq(t, "package", merged.URLTemplate)
	assert.Eq(t, "https://example.com/latest", merged.LatestURL)
	assert.Eq(t, "text", merged.LatestFormat)
	assert.Eq(t, map[string]string{"windows": "win32"}, merged.OSMap)
	assert.Eq(t, map[string]string{"amd64": "x64"}, merged.ArchMap)
	assert.Eq(t, map[string]string{"windows": ".exe"}, merged.ExtMap)
	assert.Eq(t, map[string]string{"musl": "-musl"}, merged.LibcMap)
	assert.Eq(t, "run-asset", merged.InstallAction)
	assert.Eq(t, []string{"install", "latest"}, merged.InstallArgs)
}

func boolPtr(v bool) *bool {
	return &v
}

func stringPtr(v string) *string {
	return &v
}

func intPtr(v int) *int {
	return &v
}
