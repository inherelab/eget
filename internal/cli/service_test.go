package cli

import (
	"bytes"
	"strings"
	"testing"

	cfgpkg "github.com/inherelab/eget/internal/config"
)

func TestInstallOptionsFromCommandsIncludeCacheDir(t *testing.T) {
	installOpts := installOptionsFromInstall(&InstallOptions{
		Tag:      "nightly",
		System:   "linux/amd64",
		To:       "~/.local/bin",
		CacheDir: "~/.cache/eget",
		File:     "tool",
		Asset:    "linux",
		Source:   true,
		All:      true,
		Quiet:    true,
		Add:      true,
		Name:     "tool",
	})
	if installOpts.CacheDir != "~/.cache/eget" {
		t.Fatalf("expected install cache dir to propagate, got %q", installOpts.CacheDir)
	}
	if installOpts.Name != "tool" {
		t.Fatalf("expected install name to propagate, got %q", installOpts.Name)
	}

	downloadOpts := installOptionsFromDownload(&DownloadOptions{
		Tag:      "nightly",
		System:   "linux/amd64",
		To:       "~/.cache/downloads",
		CacheDir: "~/.cache/eget",
		File:     "tool",
		Asset:    "linux",
		Source:   true,
		All:      true,
		Quiet:    true,
	})
	if downloadOpts.CacheDir != "~/.cache/eget" {
		t.Fatalf("expected download cache dir to propagate, got %q", downloadOpts.CacheDir)
	}
	if !downloadOpts.DownloadOnly {
		t.Fatal("expected download options to force DownloadOnly")
	}

	addOpts := installOptionsFromAdd(&AddOptions{
		Name:     "tool",
		Tag:      "nightly",
		System:   "linux/amd64",
		To:       "~/.local/bin",
		CacheDir: "~/.cache/eget",
		File:     "tool",
		Asset:    "linux",
		Source:   true,
		All:      true,
		Quiet:    true,
	})
	if addOpts.CacheDir != "~/.cache/eget" {
		t.Fatalf("expected add cache dir to propagate, got %q", addOpts.CacheDir)
	}

	updateOpts := installOptionsFromUpdate(&UpdateOptions{
		Tag:      "nightly",
		System:   "linux/amd64",
		To:       "~/.local/bin",
		CacheDir: "~/.cache/eget",
		Asset:    "linux",
		Source:   true,
		Quiet:    true,
	})
	if updateOpts.CacheDir != "~/.cache/eget" {
		t.Fatalf("expected update cache dir to propagate, got %q", updateOpts.CacheDir)
	}
}

func TestPrintConfigListIncludesHeaderComment(t *testing.T) {
	cfg := cfgpkg.NewFile()
	target := "~/.local/bin"
	cfg.Global.Target = &target

	var out bytes.Buffer
	printConfigList(&out, "testdata/eget.toml", true, cfg)

	got := out.String()
	if !strings.Contains(got, "# testdata/eget.toml, exists: true") {
		t.Fatalf("expected header comment, got %q", got)
	}
	if !strings.Contains(got, "[global]") {
		t.Fatalf("expected global section, got %q", got)
	}
}
