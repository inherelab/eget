package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

type fakeInstalledStoreWithLoad struct {
	cfg         *storepkg.Config
	removeCalls []string
}

func (f *fakeInstalledStoreWithLoad) Load() (*storepkg.Config, error) {
	return f.cfg, nil
}

func (f *fakeInstalledStoreWithLoad) Remove(target string) error {
	f.removeCalls = append(f.removeCalls, target)
	return nil
}

func TestUninstallWithPurgeRemovesPackageConfig(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "fzf")
	if err := os.WriteFile(binPath, []byte("bin"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	store := &fakeInstalledStoreWithLoad{
		cfg: &storepkg.Config{
			Installed: map[string]storepkg.Entry{
				"junegunn/fzf": {
					Repo:           "junegunn/fzf",
					ExtractedFiles: []string{binPath},
				},
			},
		},
	}
	cfg := cfgpkg.NewFile()
	cfg.Packages["fzf"] = cfgpkg.Section{Repo: util.StringPtr("junegunn/fzf")}
	saved := false
	svc := UninstallService{
		Store: store,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
		SaveConfig: func(file *cfgpkg.File) error {
			saved = true
			return nil
		},
	}

	_, err := svc.UninstallWithOptions("fzf", UninstallOptions{Purge: true})
	if err != nil {
		t.Fatalf("uninstall with purge: %v", err)
	}

	if _, ok := cfg.Packages["fzf"]; ok {
		t.Fatalf("expected packages.fzf to be removed, got %#v", cfg.Packages)
	}
	if !saved {
		t.Fatal("expected config to be saved")
	}
}

func TestUninstallWithPurgeRemovesUniqueRepoMatchedPackageConfig(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "fd")
	if err := os.WriteFile(binPath, []byte("bin"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	store := &fakeInstalledStoreWithLoad{
		cfg: &storepkg.Config{
			Installed: map[string]storepkg.Entry{
				"sharkdp/fd": {
					Repo:           "sharkdp/fd",
					ExtractedFiles: []string{binPath},
				},
			},
		},
	}
	cfg := cfgpkg.NewFile()
	cfg.Packages["fd"] = cfgpkg.Section{Repo: util.StringPtr("sharkdp/fd")}
	svc := UninstallService{
		Store: store,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
		SaveConfig: func(file *cfgpkg.File) error {
			return nil
		},
	}

	result, err := svc.UninstallWithOptions("sharkdp/fd", UninstallOptions{Purge: true})
	if err != nil {
		t.Fatalf("uninstall repo with purge: %v", err)
	}

	assert.Eq(t, "fd", result.PurgedConfig)
	if _, ok := cfg.Packages["fd"]; ok {
		t.Fatalf("expected packages.fd to be removed, got %#v", cfg.Packages)
	}
}

func TestUninstallWithPurgeKeepsConfigWhenRepoMatchIsAmbiguous(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "greq")
	if err := os.WriteFile(binPath, []byte("bin"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	store := &fakeInstalledStoreWithLoad{
		cfg: &storepkg.Config{
			Installed: map[string]storepkg.Entry{
				"gookit/greq": {
					Repo:           "gookit/greq",
					ExtractedFiles: []string{binPath},
				},
			},
		},
	}
	cfg := cfgpkg.NewFile()
	cfg.Packages["greq"] = cfgpkg.Section{Repo: util.StringPtr("gookit/greq")}
	cfg.Packages["gbench"] = cfgpkg.Section{Repo: util.StringPtr("gookit/greq")}
	saved := false
	svc := UninstallService{
		Store: store,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
		SaveConfig: func(file *cfgpkg.File) error {
			saved = true
			return nil
		},
	}

	result, err := svc.UninstallWithOptions("gookit/greq", UninstallOptions{Purge: true})
	if err != nil {
		t.Fatalf("uninstall ambiguous repo with purge: %v", err)
	}

	assert.Eq(t, "", result.PurgedConfig)
	assert.False(t, saved)
	if _, ok := cfg.Packages["greq"]; !ok {
		t.Fatal("expected packages.greq to remain")
	}
	if _, ok := cfg.Packages["gbench"]; !ok {
		t.Fatal("expected packages.gbench to remain")
	}
}

func TestUninstallPackageRemovesRecordedFilesAndInstalledEntry(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "fzf")
	if err := os.WriteFile(binPath, []byte("bin"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	store := &fakeInstalledStoreWithLoad{
		cfg: &storepkg.Config{
			Installed: map[string]storepkg.Entry{
				"junegunn/fzf": {
					Repo:           "junegunn/fzf",
					ExtractedFiles: []string{binPath},
				},
			},
		},
	}
	svc := UninstallService{
		Store: store,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["fzf"] = cfgpkg.Section{
				Repo: util.StringPtr("junegunn/fzf"),
			}
			return cfg, nil
		},
	}

	result, err := svc.Uninstall("fzf")
	if err != nil {
		t.Fatalf("uninstall package: %v", err)
	}
	if result.Repo != "junegunn/fzf" {
		t.Fatalf("expected repo junegunn/fzf, got %#v", result)
	}
	if len(result.RemovedFiles) != 1 || result.RemovedFiles[0] != binPath {
		t.Fatalf("expected removed file %q, got %#v", binPath, result.RemovedFiles)
	}
	if len(store.removeCalls) != 1 || store.removeCalls[0] != "junegunn/fzf" {
		t.Fatalf("expected installed record removal for junegunn/fzf, got %#v", store.removeCalls)
	}
	if _, err := os.Stat(binPath); !os.IsNotExist(err) {
		t.Fatalf("expected file %q to be removed, stat err=%v", binPath, err)
	}
}

func TestUninstallPackageUsesPackageInstalledKeyWhenPresent(t *testing.T) {
	tmp := t.TempDir()
	greqPath := filepath.Join(tmp, "greq.exe")
	gbenchPath := filepath.Join(tmp, "gbench.exe")
	if err := os.WriteFile(greqPath, []byte("greq"), 0o755); err != nil {
		t.Fatalf("write greq bin: %v", err)
	}
	if err := os.WriteFile(gbenchPath, []byte("gbench"), 0o755); err != nil {
		t.Fatalf("write gbench bin: %v", err)
	}

	store := &fakeInstalledStoreWithLoad{
		cfg: &storepkg.Config{
			Installed: map[string]storepkg.Entry{
				"greq": {
					Repo:           "gookit/greq",
					Target:         "gookit/greq",
					ExtractedFiles: []string{greqPath},
				},
				"gbench": {
					Repo:           "gookit/greq",
					Target:         "gookit/greq",
					ExtractedFiles: []string{gbenchPath},
				},
			},
		},
	}
	svc := UninstallService{
		Store: store,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["greq"] = cfgpkg.Section{Repo: util.StringPtr("gookit/greq")}
			cfg.Packages["gbench"] = cfgpkg.Section{Repo: util.StringPtr("gookit/greq")}
			return cfg, nil
		},
	}

	result, err := svc.Uninstall("greq")
	if err != nil {
		t.Fatalf("uninstall package: %v", err)
	}

	if result.Repo != "gookit/greq" {
		t.Fatalf("expected repo gookit/greq, got %#v", result)
	}
	if len(store.removeCalls) != 1 || store.removeCalls[0] != "greq" {
		t.Fatalf("expected installed record removal for greq, got %#v", store.removeCalls)
	}
	if _, err := os.Stat(greqPath); !os.IsNotExist(err) {
		t.Fatalf("expected greq file to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(gbenchPath); err != nil {
		t.Fatalf("expected gbench file to remain, stat err=%v", err)
	}
}

func TestUninstallRepoAcceptsDirectRepoTarget(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "rg")
	if err := os.WriteFile(binPath, []byte("bin"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	store := &fakeInstalledStoreWithLoad{
		cfg: &storepkg.Config{
			Installed: map[string]storepkg.Entry{
				"BurntSushi/ripgrep": {
					Repo:           "BurntSushi/ripgrep",
					ExtractedFiles: []string{binPath},
				},
			},
		},
	}
	svc := UninstallService{
		Store: store,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
	}

	result, err := svc.Uninstall("BurntSushi/ripgrep")
	if err != nil {
		t.Fatalf("uninstall repo: %v", err)
	}
	if result.Repo != "BurntSushi/ripgrep" {
		t.Fatalf("expected repo BurntSushi/ripgrep, got %#v", result)
	}
}

func TestUninstallAcceptsInstalledOnlyPackageKey(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "gbench.exe")
	if err := os.WriteFile(binPath, []byte("bin"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	store := &fakeInstalledStoreWithLoad{
		cfg: &storepkg.Config{
			Installed: map[string]storepkg.Entry{
				"gbench": {
					Repo:           "gookit/greq",
					Target:         "gookit/greq",
					ExtractedFiles: []string{binPath},
				},
			},
		},
	}
	svc := UninstallService{
		Store: store,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
	}

	result, err := svc.Uninstall("gbench")
	if err != nil {
		t.Fatalf("uninstall installed-only package key: %v", err)
	}
	if result.Repo != "gookit/greq" {
		t.Fatalf("expected repo gookit/greq, got %#v", result)
	}
	if len(store.removeCalls) != 1 || store.removeCalls[0] != "gbench" {
		t.Fatalf("expected installed record removal for gbench, got %#v", store.removeCalls)
	}
	if _, err := os.Stat(binPath); !os.IsNotExist(err) {
		t.Fatalf("expected file %q to be removed, stat err=%v", binPath, err)
	}
}

func TestUninstallFindsLegacyRepoKeyByRepoName(t *testing.T) {
	tmp := t.TempDir()
	installerPath := filepath.Join(tmp, "OpenHuman_0.54.0_x64-setup.exe")
	if err := os.WriteFile(installerPath, []byte("bin"), 0o755); err != nil {
		t.Fatalf("write installer: %v", err)
	}

	store := &fakeInstalledStoreWithLoad{
		cfg: &storepkg.Config{
			Installed: map[string]storepkg.Entry{
				"tinyhumansai/openhuman": {
					Repo:           "tinyhumansai/openhuman",
					Target:         "tinyhumansai/openhuman",
					ExtractedFiles: []string{installerPath},
				},
			},
		},
	}
	svc := UninstallService{
		Store: store,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
	}

	result, err := svc.Uninstall("openhuman")
	if err != nil {
		t.Fatalf("uninstall legacy repo key by name: %v", err)
	}
	if result.Repo != "tinyhumansai/openhuman" {
		t.Fatalf("expected repo tinyhumansai/openhuman, got %#v", result)
	}
	if len(store.removeCalls) != 1 || store.removeCalls[0] != "tinyhumansai/openhuman" {
		t.Fatalf("expected installed record removal for repo key, got %#v", store.removeCalls)
	}
}

func TestUninstallPreservesGUIInstallerMetadata(t *testing.T) {
	store := &fakeInstalledStoreWithLoad{
		cfg: &storepkg.Config{
			Installed: map[string]storepkg.Entry{
				"ansxuman/Clauge": {
					Repo:        "ansxuman/Clauge",
					IsGUI:       true,
					InstallMode: install.InstallModeInstaller,
				},
			},
		},
	}
	svc := UninstallService{
		Store: store,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["Clauge"] = cfgpkg.Section{Repo: util.StringPtr("ansxuman/Clauge")}
			return cfg, nil
		},
	}

	result, err := svc.Uninstall("Clauge")
	if err != nil {
		t.Fatalf("uninstall GUI installer: %v", err)
	}
	if !result.IsGUI || result.InstallMode != install.InstallModeInstaller {
		t.Fatalf("expected GUI installer metadata, got %#v", result)
	}
	if len(result.RemovedFiles) != 0 {
		t.Fatalf("expected no removed files, got %#v", result.RemovedFiles)
	}
}

func TestUninstallPortableGUIRemovesEmptyInstallDirectory(t *testing.T) {
	tmp := t.TempDir()
	appDir := filepath.Join(tmp, "PicoClaw")
	binPath := filepath.Join(appDir, "picoclaw.exe")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	if err := os.WriteFile(binPath, []byte("bin"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	store := &fakeInstalledStoreWithLoad{
		cfg: &storepkg.Config{
			Installed: map[string]storepkg.Entry{
				"sipeed/picoclaw": {
					Repo:           "sipeed/picoclaw",
					IsGUI:          true,
					InstallMode:    install.InstallModePortable,
					ExtractedFiles: []string{binPath},
					Options:        map[string]any{"output": appDir},
				},
			},
		},
	}
	svc := UninstallService{
		Store: store,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
	}

	if _, err := svc.Uninstall("sipeed/picoclaw"); err != nil {
		t.Fatalf("uninstall portable GUI: %v", err)
	}
	if _, err := os.Stat(appDir); !os.IsNotExist(err) {
		t.Fatalf("expected empty portable GUI install dir to be removed, stat err=%v", err)
	}
}

func TestUninstallPortableGUIKeepsSharedGuiTarget(t *testing.T) {
	tmp := t.TempDir()
	guiTarget := filepath.Join(tmp, "Programs")
	appDir := filepath.Join(guiTarget, "PicoClaw")
	binPath := filepath.Join(appDir, "picoclaw.exe")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	if err := os.WriteFile(binPath, []byte("bin"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	store := &fakeInstalledStoreWithLoad{
		cfg: &storepkg.Config{
			Installed: map[string]storepkg.Entry{
				"sipeed/picoclaw": {
					Repo:           "sipeed/picoclaw",
					IsGUI:          true,
					InstallMode:    install.InstallModePortable,
					ExtractedFiles: []string{binPath},
					Options: map[string]any{
						"output":     filepath.Join(tmp, "bin"),
						"gui_target": guiTarget,
					},
				},
			},
		},
	}
	svc := UninstallService{
		Store: store,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
	}

	if _, err := svc.Uninstall("sipeed/picoclaw"); err != nil {
		t.Fatalf("uninstall portable GUI: %v", err)
	}
	if _, err := os.Stat(appDir); !os.IsNotExist(err) {
		t.Fatalf("expected app subdir to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(guiTarget); err != nil {
		t.Fatalf("expected shared gui target to remain, stat err=%v", err)
	}
}

func TestUninstallWithPurgeRemovesConfigOnlyPackage(t *testing.T) {
	store := &fakeInstalledStoreWithLoad{
		cfg: &storepkg.Config{Installed: map[string]storepkg.Entry{}},
	}
	cfg := cfgpkg.NewFile()
	cfg.Packages["Clauge"] = cfgpkg.Section{Repo: util.StringPtr("owner/clauge")}
	saved := false
	svc := UninstallService{
		Store: store,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
		SaveConfig: func(*cfgpkg.File) error {
			saved = true
			return nil
		},
	}

	result, err := svc.UninstallWithOptions("Clauge", UninstallOptions{Purge: true})
	assert.NoErr(t, err)
	assert.Eq(t, "Clauge", result.PurgedConfig)
	assert.Eq(t, 0, len(result.RemovedFiles))
	assert.True(t, saved)
	assert.Eq(t, 0, len(store.removeCalls))
	if _, ok := cfg.Packages["Clauge"]; ok {
		t.Fatalf("expected packages.Clauge to be removed, got %#v", cfg.Packages)
	}

	// Without --purge a missing install record is still an error.
	if _, err := svc.Uninstall("Clauge"); err == nil {
		t.Fatal("expected uninstall to fail without --purge")
	}
}

func TestUninstallWithPurgeRejectsUnknownTarget(t *testing.T) {
	svc := UninstallService{
		Store: &fakeInstalledStoreWithLoad{
			cfg: &storepkg.Config{Installed: map[string]storepkg.Entry{}},
		},
		LoadConfig: func() (*cfgpkg.File, error) { return cfgpkg.NewFile(), nil },
	}

	_, err := svc.UninstallWithOptions("owner/nope", UninstallOptions{Purge: true})
	assert.Err(t, err)
	assert.StrContains(t, err.Error(), "neither installed nor configured")
}

func TestUninstallFailsWhenInstalledEntryMissing(t *testing.T) {
	store := &fakeInstalledStoreWithLoad{
		cfg: &storepkg.Config{
			Installed: map[string]storepkg.Entry{},
		},
	}
	svc := UninstallService{
		Store: store,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["fzf"] = cfgpkg.Section{
				Repo: util.StringPtr("junegunn/fzf"),
			}
			return cfg, nil
		},
	}

	if _, err := svc.Uninstall("fzf"); err == nil {
		t.Fatal("expected uninstall to fail without installed entry")
	}
}
