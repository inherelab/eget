package app

import (
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

func TestUpdatePackageUpdatesOutdatedManagedPackage(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Global.Target = util.StringPtr("~/bin")
			cfg.Packages["fzf"] = cfgpkg.Section{
				Repo:   util.StringPtr("junegunn/fzf"),
				Target: util.StringPtr("~/.local/bin"),
				System: util.StringPtr("linux/amd64"),
				Tag:    util.StringPtr("nightly"),
			}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"junegunn/fzf": {Repo: "junegunn/fzf", Tag: "v0.50.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			repo, sourcePath := target.Repo, target.SourcePath
			assert.Eq(t, "junegunn/fzf", repo)
			assert.Eq(t, "", sourcePath)
			return LatestInfo{Tag: "v0.51.0"}, nil
		},
	}

	cli := install.Options{Tag: "v1.0.0", Quiet: true}
	if _, err := svc.UpdatePackage("fzf", cli); err != nil {
		t.Fatalf("update package: %v", err)
	}

	if len(installer.targets) != 1 || installer.targets[0] != "fzf" {
		t.Fatalf("expected installer to resolve managed package name, got %#v", installer.targets)
	}
	if installer.options[0].Output != "" {
		t.Fatalf("expected update service to leave config merging to installer, got output %q", installer.options[0].Output)
	}
	if installer.options[0].Tag != "v1.0.0" || !installer.options[0].Quiet {
		t.Fatalf("expected raw cli options to pass through, got %#v", installer.options[0])
	}
}

func TestUpdatePackageUpdatesTemplateManagedPackage(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["claude"] = cfgpkg.Section{
				Repo:        util.StringPtr("template:claude"),
				LatestURL:   util.StringPtr("https://example.com/latest"),
				URLTemplate: util.StringPtr("https://example.com/{version}/{os}-{arch}/claude{ext}"),
			}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"claude": {Repo: "template:claude", Target: "template:claude", Tag: "1.2.3"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			assert.Eq(t, "claude", target.Name)
			assert.Eq(t, "template:claude", target.Repo)
			assert.Eq(t, "https://example.com/latest", *target.Package.LatestURL)
			return LatestInfo{Tag: "1.2.4"}, nil
		},
	}

	_, err := svc.UpdatePackage("claude", install.Options{})
	assert.NoErr(t, err)
	assert.Eq(t, []string{"claude"}, installer.targets)
	assert.Eq(t, install.OperationUpdate, installer.options[0].Operation)
	assert.Eq(t, "1.2.3", installer.options[0].CurrentVersion)
	assert.Eq(t, "1.2.4", installer.options[0].TargetVersion)
}

func TestUpdatePackageSkipsUpToDateManagedPackage(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["fzf"] = cfgpkg.Section{Repo: util.StringPtr("junegunn/fzf")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"junegunn/fzf": {Repo: "junegunn/fzf", Tag: "v0.50.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			repo := target.Repo
			assert.Eq(t, "junegunn/fzf", repo)
			return LatestInfo{Tag: "v0.50.0"}, nil
		},
	}

	_, err := svc.UpdatePackage("fzf", install.Options{})
	assert.NoErr(t, err)
	assert.Eq(t, 0, len(installer.targets))
}

func TestUpdatePackageRejectsUnknownDirectTargetWithInstallHint(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{}}, nil
		},
	}

	_, err := svc.UpdatePackage("sourceforge:winmerge", install.Options{})
	if err == nil || !strings.Contains(err.Error(), "use install") {
		t.Fatalf("expected use install hint, got %v", err)
	}
	assert.Eq(t, 0, len(installer.targets))
}

func TestUpdatePackageUpdatesInstalledOnlySourceForgeTarget(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"sourceforge:keepass": {
					Repo: "sourceforge:keepass",
					Tag:  "2.58",
					Options: map[string]any{
						"source_path":      "KeePass 2.x",
						"asset":            []string{"zip", "^REG:Source"},
						"extract_file":     "KeePass.exe",
						"strip_components": 1,
						"rename_files":     map[string]string{"KeePass.exe": "keepass.exe"},
					},
				},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			repo, sourcePath := target.Repo, target.SourcePath
			assert.Eq(t, "sourceforge:keepass", repo)
			assert.Eq(t, "KeePass 2.x", sourcePath)
			return LatestInfo{Tag: "2.59"}, nil
		},
	}

	_, err := svc.UpdatePackage("sourceforge:keepass", install.Options{})
	assert.NoErr(t, err)
	assert.Eq(t, []string{"sourceforge:keepass"}, installer.targets)
	assert.Eq(t, "KeePass 2.x", installer.options[0].SourcePath)
	assert.Eq(t, []string{"zip", "^REG:Source"}, installer.options[0].Asset)
	assert.Eq(t, "KeePass.exe", installer.options[0].ExtractFile)
	assert.Eq(t, 1, installer.options[0].StripComponents)
	assert.Eq(t, map[string]string{"KeePass.exe": "keepass.exe"}, installer.options[0].RenameFiles)
}

func TestUpdatePackageRestoresTemplateOptionsFromInstalledEntry(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"claude": {
					Repo:    "template:claude",
					Target:  "template:claude",
					Tag:     "1.2.3",
					Version: "1.2.3",
					Options: map[string]any{
						"latest_url":            "https://example.com/latest",
						"latest_format":         "text",
						"url_template":          "https://example.com/{version}/{os}-{arch}/claude{ext}",
						"os_map":                map[string]string{"windows": "win32"},
						"arch_map":              map[string]string{"amd64": "x64"},
						"ext_map":               map[string]string{"windows": ".exe"},
						"checksum_url_template": "https://example.com/{version}/manifest.json",
						"checksum_format":       "json",
						"checksum_json_path":    "platforms.{os}-{arch}.checksum",
						"install_action":        "run-asset",
						"install_args":          []string{"install", "latest"},
						"install_mode":          "installer",
					},
				},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			repo, sourcePath := target.Repo, target.SourcePath
			assert.Eq(t, "template:claude", repo)
			assert.Eq(t, "", sourcePath)
			return LatestInfo{Tag: "1.2.4"}, nil
		},
	}

	_, err := svc.UpdatePackage("claude", install.Options{})
	assert.NoErr(t, err)
	assert.Eq(t, []string{"template:claude"}, installer.targets)
	opts := installer.options[0]
	assert.Eq(t, "https://example.com/latest", opts.URLTemplate.LatestURL)
	assert.Eq(t, "https://example.com/{version}/{os}-{arch}/claude{ext}", opts.URLTemplate.URLTemplate)
	assert.Eq(t, map[string]string{"windows": "win32"}, opts.URLTemplate.OSMap)
	assert.Eq(t, map[string]string{"amd64": "x64"}, opts.URLTemplate.ArchMap)
	assert.Eq(t, map[string]string{"windows": ".exe"}, opts.URLTemplate.ExtMap)
	assert.Eq(t, "https://example.com/{version}/manifest.json", opts.URLTemplate.ChecksumURLTemplate)
	assert.Eq(t, "json", opts.URLTemplate.ChecksumFormat)
	assert.Eq(t, "platforms.{os}-{arch}.checksum", opts.URLTemplate.ChecksumJSONPath)
	assert.Eq(t, "run-asset", opts.URLTemplate.InstallAction)
	assert.Eq(t, []string{"install", "latest"}, opts.URLTemplate.InstallArgs)
	assert.Eq(t, install.InstallModeInstaller, opts.InstallMode)
}

func TestUpdatePackageRestoresPrereleaseOptionFromInstalledEntry(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"pkgforge/aeris": {
					Repo: "pkgforge/aeris",
					Tag:  "v0.11.0-beta",
					Options: map[string]any{
						"prerelease": true,
					},
				},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			assert.Eq(t, "pkgforge/aeris", target.Repo)
			assert.True(t, target.Prerelease)
			return LatestInfo{Tag: "v0.12.0-beta"}, nil
		},
	}

	_, err := svc.UpdatePackage("pkgforge/aeris", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, []string{"pkgforge/aeris"}, installer.targets)
	assert.True(t, installer.options[0].Prerelease)
}

func TestUpdatePackageRestoresCustomInstalledName(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"Apocrypha.AppImage": {
					Repo:   "Jeagermeister/Apocrypha",
					Target: "Jeagermeister/Apocrypha",
					Tag:    "v0.5.0",
					Options: map[string]any{
						"prerelease": true,
						"asset":      []string{"AppImage"},
					},
				},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			assert.True(t, target.Prerelease)
			return LatestInfo{Tag: "v0.6.0"}, nil
		},
	}

	_, err := svc.UpdatePackage("Apocrypha.AppImage", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, []string{"Jeagermeister/Apocrypha"}, installer.targets)
	assert.Eq(t, "Apocrypha.AppImage", installer.options[0].Name)
	assert.True(t, installer.options[0].Prerelease)
}

func TestUpdatePackageClearsInstalledVersionTagWhenUpdatingToNewLatest(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"AandStep/ResultV": {
					Repo: "AandStep/ResultV",
					Tag:  "v3.2.5",
					Options: map[string]any{
						"tag":   "v3.2.5",
						"asset": []string{"AppImage"},
					},
				},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			assert.Eq(t, "AandStep/ResultV", target.Repo)
			assert.Eq(t, "", target.Tag)
			return LatestInfo{Tag: "v3.2.6"}, nil
		},
	}

	_, err := svc.UpdatePackage("ResultV", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, []string{"AandStep/ResultV"}, installer.targets)
	assert.Eq(t, "", installer.options[0].Tag)
	assert.Eq(t, []string{"AppImage"}, installer.options[0].Asset)
	assert.Eq(t, "v3.2.5", installer.options[0].CurrentVersion)
	assert.Eq(t, "v3.2.6", installer.options[0].TargetVersion)
}

func TestUpdatePackageKeepsInstalledChannelTag(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"pkgforge/aeris": {
					Repo:      "pkgforge/aeris",
					Tag:       "nightly",
					Asset:     "aeris.tar.xz",
					AssetSize: 12,
					Options: map[string]any{
						"tag": "nightly",
					},
				},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			assert.Eq(t, "pkgforge/aeris", target.Repo)
			assert.Eq(t, "nightly", target.Tag)
			return LatestInfo{Tag: "nightly", AssetName: "aeris.tar.xz", AssetSize: 13}, nil
		},
	}

	_, err := svc.UpdatePackage("aeris", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, []string{"pkgforge/aeris"}, installer.targets)
	assert.Eq(t, "nightly", installer.options[0].Tag)
}

func TestUpdatePackageKeepsInstalledVersionTagWithPolicy(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"AandStep/ResultV": {
					Repo:      "AandStep/ResultV",
					Tag:       "v3.2.5",
					TagPolicy: "tag",
					Asset:     "ResultV.AppImage",
					AssetSize: 12,
					Options: map[string]any{
						"tag":        "v3.2.5",
						"tag_policy": "tag",
					},
				},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			assert.Eq(t, "AandStep/ResultV", target.Repo)
			assert.Eq(t, "v3.2.5", target.Tag)
			return LatestInfo{Tag: "v3.2.5", AssetName: "ResultV.AppImage", AssetSize: 13}, nil
		},
	}

	_, err := svc.UpdatePackage("ResultV", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, []string{"AandStep/ResultV"}, installer.targets)
	assert.Eq(t, "v3.2.5", installer.options[0].Tag)
}

func TestUpdatePackageRejectsUnknownPlainWords(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{}}, nil
		},
	}

	for _, name := range []string{"gitlab", "not-managed", "foo/bar/baz"} {
		t.Run(name, func(t *testing.T) {
			_, err := svc.UpdatePackage(name, install.Options{})
			if err == nil || !strings.Contains(err.Error(), "use install") {
				t.Fatalf("expected unknown package error for %q, got %v", name, err)
			}
		})
	}
	assert.Eq(t, 0, len(installer.targets))
}

func TestUpdatePackageWithAppInstallerKeepsManagedConfigMerge(t *testing.T) {
	cfg := mustLoadFromString(t, `
[global]
target = "~/.local/bin"

["junegunn/fzf"]
system = "linux/amd64"

[packages.fzf]
repo = "junegunn/fzf"
target = "D:/Tools/fzf"
tag = "nightly"
asset_filters = ["linux"]
`)
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/junegunn/fzf/releases/download/nightly/fzf.tar.gz",
			Tool:           "fzf",
			ExtractedFiles: []string{"./fzf"},
		},
	}
	installSvc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}
	updateSvc := UpdateService{
		Install: installSvc,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"junegunn/fzf": {Repo: "junegunn/fzf", Tag: "v0.50.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			repo := target.Repo
			assert.Eq(t, "junegunn/fzf", repo)
			return LatestInfo{Tag: "nightly"}, nil
		},
	}

	if _, err := updateSvc.UpdatePackage("fzf", install.Options{}); err != nil {
		t.Fatalf("update package: %v", err)
	}

	if runner.target != "junegunn/fzf" {
		t.Fatalf("expected installer to resolve repo target, got %q", runner.target)
	}
	if runner.opts.Output != "D:/Tools/fzf" {
		t.Fatalf("expected package target to be merged by installer, got %q", runner.opts.Output)
	}
	if runner.opts.System != "linux/amd64" {
		t.Fatalf("expected repo system to be merged by installer, got %q", runner.opts.System)
	}
	if runner.opts.Tag != "nightly" {
		t.Fatalf("expected package tag to be merged by installer, got %q", runner.opts.Tag)
	}
	if len(runner.opts.Asset) != 1 || runner.opts.Asset[0] != "linux" {
		t.Fatalf("expected package asset filters to be merged by installer, got %#v", runner.opts.Asset)
	}
}

func TestUpdatePackageWithAppInstallerIgnoresManagedVersionTag(t *testing.T) {
	cfg := mustLoadFromString(t, `
[packages.ResultV]
repo = "AandStep/ResultV"
tag = "v3.2.5"
asset_filters = ["AppImage"]
`)
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/AandStep/ResultV/releases/download/v3.2.6/ResultV-3.2.6-x86_64.AppImage",
			Tool:           "ResultV",
			ExtractedFiles: []string{"./ResultV.AppImage"},
		},
	}
	installSvc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}
	updateSvc := UpdateService{
		Install: installSvc,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"AandStep/ResultV": {Repo: "AandStep/ResultV", Tag: "v3.2.5"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			assert.Eq(t, "AandStep/ResultV", target.Repo)
			assert.Eq(t, "", target.Tag)
			return LatestInfo{Tag: "v3.2.6"}, nil
		},
	}

	_, err := updateSvc.UpdatePackage("ResultV", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, "AandStep/ResultV", runner.target)
	assert.Eq(t, "", runner.opts.Tag)
	assert.Eq(t, []string{"AppImage"}, runner.opts.Asset)
	assert.Eq(t, "v3.2.5", runner.opts.CurrentVersion)
	assert.Eq(t, "v3.2.6", runner.opts.TargetVersion)
}

func TestUpdatePackageWithAppInstallerKeepsManagedVersionTagPolicy(t *testing.T) {
	cfg := mustLoadFromString(t, `
[packages.ResultV]
repo = "AandStep/ResultV"
tag = "v3.2.5"
tag_policy = "tag"
asset_filters = ["AppImage"]
`)
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/AandStep/ResultV/releases/download/v3.2.5/ResultV.AppImage",
			Tool:           "ResultV",
			ExtractedFiles: []string{"./ResultV.AppImage"},
		},
	}
	installSvc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}
	updateSvc := UpdateService{
		Install: installSvc,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"AandStep/ResultV": {Repo: "AandStep/ResultV", Tag: "v3.2.5", Asset: "ResultV.AppImage", AssetSize: 12},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			assert.Eq(t, "AandStep/ResultV", target.Repo)
			assert.Eq(t, "v3.2.5", target.Tag)
			return LatestInfo{Tag: "v3.2.5", AssetName: "ResultV.AppImage", AssetSize: 13}, nil
		},
	}

	_, err := updateSvc.UpdatePackage("ResultV", install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, "AandStep/ResultV", runner.target)
	assert.Eq(t, "v3.2.5", runner.opts.Tag)
	assert.Eq(t, "tag", runner.opts.TagPolicy)
}
