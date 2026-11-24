package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gookit/cliui"
	"github.com/gookit/goutil/x/assert"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	"github.com/inherelab/eget/internal/client"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

func TestHandleListOutdatedPrintsOnlyOutdatedInstalledPackages(t *testing.T) {
	svc := &cliService{
		listService: app.ListService{
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["fzf"] = cfgpkg.Section{Repo: util.StringPtr("junegunn/fzf")}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{
					Installed: map[string]storepkg.Entry{
						"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
						"junegunn/fzf":       {Repo: "junegunn/fzf", Tag: "v0.50.0"},
					},
				}, nil
			},
		},
	}
	publishedAt := time.Date(2026, 4, 21, 14, 10, 17, 0, time.UTC)
	svc.listService.LatestInfo = func(target app.LatestCheckTarget) (app.LatestInfo, error) {
		repo := target.Repo
		switch repo {
		case "BurntSushi/ripgrep":
			return app.LatestInfo{Tag: "v14.0.0", PublishedAt: publishedAt}, nil
		case "junegunn/fzf":
			return app.LatestInfo{Tag: "v0.50.0"}, nil
		default:
			return app.LatestInfo{}, nil
		}
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleList(&ListOptions{Outdated: true})
	if err != nil {
		t.Fatalf("handle list outdated: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Outdated Packages (1):") {
		t.Fatalf("expected outdated packages title, got %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "latest version") {
		t.Fatalf("expected last_version column in output, got %q", got)
	}
	if !strings.Contains(got, "Published At") {
		t.Fatalf("expected published at column in output, got %q", got)
	}
	if !strings.Contains(got, "2026-04-21 14:10:17") {
		t.Fatalf("expected published time in output, got %q", got)
	}
	if strings.Contains(got, "2026-04-21T14:10:17") {
		t.Fatalf("expected published time with a space between date and time, got %q", got)
	}
	if strings.Contains(got, " yes ") || strings.Contains(got, " no ") {
		t.Fatalf("expected Installed column values to be removed, got %q", got)
	}
	if !strings.Contains(got, "BurntSushi/ripgrep") {
		t.Fatalf("expected outdated repo in output, got %q", got)
	}
	if !strings.Contains(got, "v14.0.0") {
		t.Fatalf("expected latest_tag in output, got %q", got)
	}
	if strings.Contains(got, "junegunn/fzf") {
		t.Fatalf("expected up-to-date repo to be omitted, got %q", got)
	}
}

func TestHandleListOutdatedPrintsCheckedInstalledCountWhenNothingOutdated(t *testing.T) {
	svc := &cliService{
		listService: app.ListService{
			LatestInfo: func(target app.LatestCheckTarget) (app.LatestInfo, error) {
				repo := target.Repo
				switch repo {
				case "gookit/gitw":
					return app.LatestInfo{Tag: "v0.3.6"}, nil
				case "sipeed/picoclaw":
					return app.LatestInfo{Tag: "v0.2.7"}, nil
				case "windirstat/windirstat":
					return app.LatestInfo{Tag: "release/v2.5.0"}, nil
				default:
					return app.LatestInfo{}, nil
				}
			},
			LoadConfig: func() (*cfgpkg.File, error) {
				return cfgpkg.NewFile(), nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{
					Installed: map[string]storepkg.Entry{
						"gookit/gitw":           {Repo: "gookit/gitw", Tag: "v0.3.6"},
						"sipeed/picoclaw":       {Repo: "sipeed/picoclaw", Tag: "v0.2.7"},
						"windirstat/windirstat": {Repo: "windirstat/windirstat", Tag: "release/v2.5.0"},
					},
				}, nil
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleList(&ListOptions{Outdated: true})
	if err != nil {
		t.Fatalf("handle list outdated: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Checked 3 packages") {
		t.Fatalf("expected checked count for all installed packages, got %q", got)
	}
	if !strings.Contains(got, "No outdated packages found") {
		t.Fatalf("expected no outdated message, got %q", got)
	}
}

func TestHandleListOutdatedPrintsSingleProxyNoticeAndCacheSummary(t *testing.T) {
	cacheDir := t.TempDir()
	repos := []string{"gookit/gitw", "sipeed/picoclaw", "windirstat/windirstat"}
	for _, repo := range repos {
		apiURL := "https://api.github.com/repos/" + repo + "/releases/latest"
		cachePath := install.APICacheFilePath(cacheDir, apiURL)
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
			t.Fatalf("mkdir cache dir: %v", err)
		}
		if err := os.WriteFile(cachePath, []byte(`{"tag_name":"v1.0.0"}`), 0o644); err != nil {
			t.Fatalf("write cache file: %v", err)
		}
	}

	svc := &cliService{
		proxyURL: "http://127.0.0.1:1081",
		listService: app.ListService{
			LatestInfo: func(target app.LatestCheckTarget) (app.LatestInfo, error) {
				repo := target.Repo
				apiURL := "https://api.github.com/repos/" + repo + "/releases/latest"
				resp, err := client.GetWithOptions(apiURL, client.Options{
					ProxyURL:        "http://127.0.0.1:1081",
					APICacheEnabled: true,
					APICacheDir:     cacheDir,
					APICacheTime:    300,
				})
				if err != nil {
					return app.LatestInfo{}, err
				}
				_ = resp.Body.Close()
				return app.LatestInfo{Tag: "v1.0.0"}, nil
			},
			LoadConfig: func() (*cfgpkg.File, error) {
				return cfgpkg.NewFile(), nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{Installed: map[string]storepkg.Entry{
					"gookit/gitw":           {Repo: "gookit/gitw", Tag: "v1.0.0"},
					"sipeed/picoclaw":       {Repo: "sipeed/picoclaw", Tag: "v1.0.0"},
					"windirstat/windirstat": {Repo: "windirstat/windirstat", Tag: "v1.0.0"},
				}}, nil
			},
		},
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	svc.stderr = &stderr
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleList(&ListOptions{Outdated: true})
	if err != nil {
		t.Fatalf("handle list outdated: %v", err)
	}

	gotErr := ccolor.ClearCode(stderr.String())
	if strings.Count(gotErr, "http_proxy for GitHub API request") != 1 {
		t.Fatalf("expected one proxy notice, got %q", gotErr)
	}
	if !strings.Contains(gotErr, "Reused api_cache files: 3") {
		t.Fatalf("expected cache summary, got %q", gotErr)
	}
}

func TestHandleListOutdatedSkipsProxyNoticeWhenGitHubAPIExcluded(t *testing.T) {
	svc := &cliService{
		proxyURL:     "http://127.0.0.1:1081",
		proxyExclude: []string{"api.github.com"},
		listService: app.ListService{
			LatestInfo: func(target app.LatestCheckTarget) (app.LatestInfo, error) {
				return app.LatestInfo{Tag: "v14.0.0"}, nil
			},
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{Installed: map[string]storepkg.Entry{
					"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
				}}, nil
			},
		},
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	svc.stderr = &stderr
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleList(&ListOptions{Outdated: true})

	assert.NoErr(t, err)
	gotErr := ccolor.ClearCode(stderr.String())
	assert.NotContains(t, gotErr, "http_proxy for GitHub API request")
}

func TestHandleListShowsUpdatedAtWhenPresent(t *testing.T) {
	installed := time.Date(2026, 11, 13, 4, 31, 34, 0, time.UTC)
	updated := time.Date(2026, 11, 22, 9, 15, 2, 0, time.UTC)
	svc := &cliService{
		listService: app.ListService{
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["linux"] = cfgpkg.Section{Repo: util.StringPtr("colitu/linux")}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{Installed: map[string]storepkg.Entry{
					"linux": {Repo: "colitu/linux", Tag: "v1.9.0", InstalledAt: installed, UpdatedAt: updated},
				}}, nil
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	if err := svc.handleList(&ListOptions{}); err != nil {
		t.Fatalf("handle list: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "2026-11-22 09:15:02") {
		t.Fatalf("expected the update time to be the last update, got %q", got)
	}
	if strings.Contains(got, "2026-11-13 04:31:34") {
		t.Fatalf("expected the install time to be replaced by the update time, got %q", got)
	}
}

func TestHandleListPrintsOnlyInstalledPackagesByDefault(t *testing.T) {
	now := time.Date(2026, 5, 5, 13, 20, 19, 0, time.FixedZone("CST", 8*60*60))
	svc := &cliService{
		listService: app.ListService{
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["chlog"] = cfgpkg.Section{Repo: util.StringPtr("gookit/gitw")}
				cfg.Packages["claude"] = cfgpkg.Section{Repo: util.StringPtr("template:claude")}
				cfg.Packages["ripgrep"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{
					Installed: map[string]storepkg.Entry{
						"gookit/gitw": {Repo: "gookit/gitw", Tag: "v0.3.6", InstalledAt: now},
						"claude":      {Repo: "template:claude", Target: "template:claude", Tag: "1.2.3", InstalledAt: now},
					},
				}, nil
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleList(&ListOptions{})
	if err != nil {
		t.Fatalf("handle list: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Installed Packages (2):") {
		t.Fatalf("expected installed packages title, got %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "name") || !strings.Contains(strings.ToLower(got), "version") {
		t.Fatalf("expected table headers in output, got %q", got)
	}
	if strings.Contains(got, " yes ") || strings.Contains(got, " no ") {
		t.Fatalf("expected Installed column values to be removed, got %q", got)
	}
	if !strings.Contains(got, "Source") || !strings.Contains(got, "Update Time") {
		t.Fatalf("expected Source and Update Time columns, got %q", got)
	}
	if !strings.Contains(got, "chlog") || !strings.Contains(got, "v0.3.6") {
		t.Fatalf("expected table row in output, got %q", got)
	}
	if !strings.Contains(got, "claude") || !strings.Contains(got, "template") {
		t.Fatalf("expected template package source in output, got %q", got)
	}
	if !strings.Contains(got, "github") || !strings.Contains(got, "2026-05-05 13:20:19") {
		t.Fatalf("expected source and update time in output, got %q", got)
	}
	if strings.Contains(got, "2026-05-05T13:20:19") {
		t.Fatalf("expected update time with a space between date and time, got %q", got)
	}
	if strings.Contains(got, "ripgrep") {
		t.Fatalf("expected default list to omit managed-only package, got %q", got)
	}
}

func TestHandleListAllPrintsManagedAndInstalledPackages(t *testing.T) {
	now := time.Date(2026, 5, 5, 13, 20, 19, 0, time.FixedZone("CST", 8*60*60))
	svc := &cliService{
		listService: app.ListService{
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["chlog"] = cfgpkg.Section{Repo: util.StringPtr("gookit/gitw")}
				cfg.Packages["ripgrep"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{
					Installed: map[string]storepkg.Entry{
						"gookit/gitw": {Repo: "gookit/gitw", Tag: "v0.3.6", InstalledAt: now},
					},
				}, nil
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleList(&ListOptions{All: true})
	if err != nil {
		t.Fatalf("handle list all: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Managed Packages (2):") {
		t.Fatalf("expected managed packages title, got %q", got)
	}
	if !strings.Contains(got, "Source") || !strings.Contains(got, "Update Time") {
		t.Fatalf("expected Source and Update Time columns, got %q", got)
	}
	if !strings.Contains(got, "chlog") || !strings.Contains(got, "ripgrep") {
		t.Fatalf("expected all list to include installed and managed-only packages, got %q", got)
	}
	if !strings.Contains(got, "v0.3.6") || !strings.Contains(got, "No-Install") {
		t.Fatalf("expected installed and non-installed versions, got %q", got)
	}
	if strings.Contains(got, " yes ") || strings.Contains(got, " no ") {
		t.Fatalf("expected Installed column values to be removed, got %q", got)
	}
	if !strings.Contains(got, "2026-05-05 13:20:19") {
		t.Fatalf("expected update time in output, got %q", got)
	}
	if strings.Contains(got, "2026-05-05T13:20:19") {
		t.Fatalf("expected update time with a space between date and time, got %q", got)
	}
}

func TestHandleListNoInstalledPrintsOnlyManagedMissingPackages(t *testing.T) {
	now := time.Date(2026, 5, 5, 13, 20, 19, 0, time.FixedZone("CST", 8*60*60))
	svc := &cliService{
		listService: app.ListService{
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.PkgTemplates["kdev"] = cfgpkg.Section{
					LatestURL:   util.StringPtr("https://example.com/{name}/latest.yaml"),
					URLTemplate: util.StringPtr("https://example.com/{name}/{version}/{name}.zip"),
				}
				cfg.Packages["chlog"] = cfgpkg.Section{Repo: util.StringPtr("gookit/gitw")}
				cfg.Packages["doccli"] = cfgpkg.Section{Repo: util.StringPtr("pkg-template:kdev:doccli")}
				cfg.Packages["ripgrep"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{
					Installed: map[string]storepkg.Entry{
						"gookit/gitw": {Repo: "gookit/gitw", Tag: "v0.3.6", InstalledAt: now},
					},
				}, nil
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleList(&ListOptions{NoInstalled: true})
	if err != nil {
		t.Fatalf("handle list no-installed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Not Installed Packages (2):") {
		t.Fatalf("expected not installed packages title, got %q", got)
	}
	if !strings.Contains(got, "ripgrep") || !strings.Contains(got, "doccli") || !strings.Contains(got, "No-Install") {
		t.Fatalf("expected managed-only package in output, got %q", got)
	}
	if !strings.Contains(got, "pkg-template") {
		t.Fatalf("expected pkg-template source in output, got %q", got)
	}
	if strings.Contains(got, "chlog") || strings.Contains(got, "v0.3.6") {
		t.Fatalf("expected installed package to be omitted, got %q", got)
	}
}

func TestPackageSourceShowsPkgTemplate(t *testing.T) {
	got := packageSource(app.ListItem{Repo: "pkg-template:kdev:doccli"})

	assert.Eq(t, "pkg-template", got)
}

func TestHandleListGUIPrintsOnlyGUIPackages(t *testing.T) {
	isGUI := true
	svc := &cliService{
		listService: app.ListService{
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["picoclaw"] = cfgpkg.Section{Repo: util.StringPtr("sipeed/picoclaw"), IsGUI: &isGUI}
				cfg.Packages["chlog"] = cfgpkg.Section{Repo: util.StringPtr("gookit/gitw")}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{Installed: map[string]storepkg.Entry{
					"sipeed/picoclaw": {Repo: "sipeed/picoclaw", Tag: "v0.2.7", IsGUI: true, InstallMode: "portable"},
					"gookit/gitw":     {Repo: "gookit/gitw", Tag: "v0.3.6"},
				}}, nil
			},
		},
	}
	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)
	err := svc.handleList(&ListOptions{GUI: true})
	if err != nil {
		t.Fatalf("handle list gui: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "GUI Packages (1):") {
		t.Fatalf("expected gui packages title, got %q", got)
	}
	if !strings.Contains(got, "picoclaw") || strings.Contains(got, "chlog") {
		t.Fatalf("expected only gui package output, got %q", got)
	}
}

func TestHandleListInfoPrintsDetails(t *testing.T) {
	now := time.Date(2026, 5, 5, 13, 20, 19, 0, time.FixedZone("CST", 8*60*60))
	svc := &cliService{
		listService: app.ListService{
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["chlog"] = cfgpkg.Section{
					Repo:   util.StringPtr("gookit/gitw"),
					Target: util.StringPtr("~/.local/bin"),
				}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{
					Installed: map[string]storepkg.Entry{
						"gookit/gitw": {
							Repo:        "gookit/gitw",
							InstalledAt: now,
							Tag:         "v0.3.6",
							Asset:       "chlog-windows-amd64.exe",
							URL:         "https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe",
						},
					},
				}, nil
			},
		},
	}

	var out bytes.Buffer
	cliui.SetOutput(&out)
	defer cliui.ResetOutput()

	err := svc.handleList(&ListOptions{Info: "chlog"})
	if err != nil {
		t.Fatalf("handle list info: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Package Info") || !strings.Contains(got, "Name") || !strings.Contains(got, "chlog") {
		t.Fatalf("expected detail output, got %q", got)
	}
	if !strings.Contains(got, "Version") || !strings.Contains(got, "v0.3.6") {
		t.Fatalf("expected version detail output, got %q", got)
	}
	if !strings.Contains(got, "URL") || !strings.Contains(got, "https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe") {
		t.Fatalf("expected detailed url output, got %q", got)
	}
	if !strings.Contains(got, "2026-05-05 13:20:19") {
		t.Fatalf("expected compact installed time in detail output, got %q", got)
	}
	if strings.Contains(got, "2026-05-05T13:20:19") || strings.Contains(got, "2026-05-05 13:20:19 +0800") {
		t.Fatalf("expected detail time with a space between date and time and no timezone offset, got %q", got)
	}
}

func TestHandleShowPrintsPackageDetails(t *testing.T) {
	installedAt := time.Date(2026, 5, 5, 13, 20, 19, 0, time.UTC)
	svc := &cliService{
		showService: app.ShowService{
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["chlog"] = cfgpkg.Section{
					Repo: util.StringPtr("gookit/gitw"),
					Desc: util.StringPtr("Git helper tools"),
				}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{Installed: map[string]storepkg.Entry{
					"gookit/gitw": {
						Repo:           "gookit/gitw",
						Target:         "gookit/gitw",
						InstalledAt:    installedAt,
						Tag:            "v0.3.6",
						Version:        "v0.3.6",
						Asset:          "chlog-windows-amd64.exe",
						URL:            "https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe",
						ExtractedFiles: []string{"D:/bin/chlog.exe"},
						RepoURL:        "https://github.com/gookit/gitw",
					},
				}}, nil
			},
		},
	}

	var out bytes.Buffer
	cliui.SetOutput(&out)
	defer cliui.ResetOutput()

	err := svc.handleShow(&ShowOptions{Target: "chlog"})
	if err != nil {
		t.Fatalf("handle show: %v", err)
	}

	got := out.String()
	assert.Contains(t, got, "Package Details")
	assert.Contains(t, got, "chlog")
	assert.Contains(t, got, "Git helper tools")
	assert.Contains(t, got, "v0.3.6")
	assert.Contains(t, got, "https://github.com/gookit/gitw")
	assert.Contains(t, got, "D:/bin/chlog.exe")
	assert.Contains(t, got, "2026-05-05 13:20:19")
}

func TestHandleListRejectsOutdatedWithInfo(t *testing.T) {
	svc := &cliService{}
	err := svc.handleList(&ListOptions{Outdated: true, Info: "chlog"})
	if err == nil {
		t.Fatal("expected conflicting list options to fail")
	}
}

func TestHandleListRejectsNoInstalledConflicts(t *testing.T) {
	svc := &cliService{}
	if err := svc.handleList(&ListOptions{NoInstalled: true, Info: "chlog"}); err == nil {
		t.Fatal("expected no-installed with info to fail")
	}
	if err := svc.handleList(&ListOptions{NoInstalled: true, Outdated: true}); err == nil {
		t.Fatal("expected no-installed with outdated to fail")
	}
}
