package app

import (
	"os"
	"path/filepath"
	"testing"

	cfgpkg "github.com/inherelab/eget/internal/config"
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
