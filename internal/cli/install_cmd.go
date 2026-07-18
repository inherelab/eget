package cli

import (
	"github.com/gookit/gcli/v3"
	"github.com/inherelab/eget/internal/install"
)

type InstallOptions struct {
	Tag              string
	System           string
	To               string
	File             string
	Asset            string
	Rename           string
	Name             string
	InstallMode      string
	StripComponents  int
	Source           bool
	Prerelease       bool
	TrackTag         bool
	All              bool
	InstallAll       bool
	GUI              bool
	Quiet            bool
	Add              bool
	FallbackVersions int
	Retries          int
	ChunkConcurrency int
	BatchConcurrency int
	Targets          []string
}

func newInstallCmd(handler CommandHandler) (*gcli.Command, func()) {
	opts := &InstallOptions{Retries: 1, ChunkConcurrency: -1, BatchConcurrency: -1}
	cmd := gcli.NewCommand("install", "Install one or more targets")
	cmd.Aliases = []string{"i", "ins"}
	cmd.Config = func(c *gcli.Command) {
		c.StrOpt(&opts.Tag, "tag", "", "", "Release tag")
		c.StrOpt(&opts.System, "system", "", "", "Target system. eg: linux/amd64")
		c.StrOpt(&opts.To, "to", "", "", "Install destination")
		c.StrOpt(&opts.File, "file", "", "", "File to extract, multi use comma split, support glob")
		c.StrOpt(&opts.Asset, "asset", "a", "", "Asset filter, multi use comma split")
		c.StrOpt(&opts.Rename, "rename", "", "", "Rename extracted files, comma separated from=to pairs")
		c.StrOpt(&opts.Name, "name", "", "", "Managed package name when used with --add")
		c.StrOpt(&opts.InstallMode, "install-mode", "imode", "", "GUI install mode: portable or installer")
		c.IntOpt(&opts.StripComponents, "strip-components", "", 0, "Strip leading archive path components when extracting all files")
		c.BoolOpt(&opts.Source, "source", "", false, "Download source archive")
		c.BoolOpt(&opts.Prerelease, "prerelease", "p", false, "Select latest release including prereleases")
		c.BoolOpt(&opts.TrackTag, "track-tag", "", false, "Track the selected release tag on updates")
		c.BoolOpt(&opts.All, "extract-all", "ea", false, "Extract all files")
		c.BoolOpt(&opts.InstallAll, "all", "", false, "Install all managed packages from config")
		c.BoolOpt(&opts.GUI, "gui", "", false, "Install as GUI application")
		c.BoolOpt(&opts.Quiet, "quiet", "", false, "Quiet output")
		c.BoolOpt(&opts.Add, "add", "", false, "Add installed repo target to managed packages")
		c.IntOpt(&opts.FallbackVersions, "fallback-versions", "", 0, "Search older SourceForge version folders when asset is missing")
		c.IntOpt(&opts.Retries, "retries", "", 1, "Download request attempts per URL")
		c.IntOpt(&opts.ChunkConcurrency, "chunk", "", -1, "HTTP Range chunk concurrency: 0 auto, 1 single connection")
		c.IntOpt(&opts.BatchConcurrency, "batch", "", -1, "Concurrent package tasks for --all: 0 auto, 1 serial")
		c.AddArg("target", "Installation target(s)", false, true)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		targetArgs := append(c.Arg("target").Strings(), args...)
		if err := validateNoFlagArgs(targetArgs); err != nil {
			return err
		}
		if err := validateRetries(opts.Retries); err != nil {
			return err
		}
		mode, err := normalizeInstallMode(opts.InstallMode)
		if err != nil {
			return err
		}
		if mode == "" && opts.GUI {
			mode = install.InstallModeInstaller
		}
		opts.InstallMode = mode
		if mode != "" {
			opts.GUI = true
		}
		opts.Targets = splitTargets(targetArgs)
		snapshot := *opts
		snapshot.Targets = append([]string(nil), opts.Targets...)
		return handler("install", &snapshot)
	}
	return cmd, func() {
		*opts = InstallOptions{Retries: 1, ChunkConcurrency: -1, BatchConcurrency: -1}
	}
}
