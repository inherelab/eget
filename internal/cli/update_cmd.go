package cli

import "github.com/gookit/gcli/v3"

type UpdateOptions struct {
	All              bool
	Check            bool
	DryRun           bool
	Interactive      bool
	Self             bool
	SelfSource       string
	Tag              string
	System           string
	To               string
	File             string
	Asset            string
	Source           bool
	Quiet            bool
	Retries          int
	ChunkConcurrency int
	BatchConcurrency int
	Targets          []string
}

func newUpdateCmd(handler CommandHandler) (*gcli.Command, func()) {
	opts := &UpdateOptions{Retries: 1, ChunkConcurrency: -1, BatchConcurrency: -1}
	cmd := gcli.NewCommand("update", "Update installed targets")
	cmd.Aliases = []string{"up"}
	cmd.Config = func(c *gcli.Command) {
		c.BoolOpt(&opts.All, "all", "A", false, "Update all managed packages")
		c.BoolOpt(&opts.Check, "check", "", false, "Check and list outdated installed packages")
		c.BoolOpt(&opts.DryRun, "dry-run", "", false, "Preview updates without changes")
		c.BoolOpt(&opts.Interactive, "interactive", "i", false, "Interactively choose packages")
		c.BoolOpt(&opts.Self, "self", "", false, "Update eget itself")
		c.StrOpt(&opts.SelfSource, "self-source", "", "", "Self update source base URL or latest.yaml URL")
		c.StrOpt(&opts.Tag, "tag", "", "", "Release tag")
		c.StrOpt(&opts.System, "system", "", "", "Target system. eg: linux/amd64")
		c.StrOpt(&opts.To, "to", "", "", "Install destination")
		c.StrOpt(&opts.File, "file", "", "", "File to extract, multi use comma split, support glob")
		c.StrOpt(&opts.Asset, "asset", "a", "", "Asset filter, multi use comma split")
		c.BoolOpt(&opts.Source, "source", "", false, "Download source archive")
		c.BoolOpt(&opts.Quiet, "quiet", "", false, "Quiet output; select first asset when multiple candidates remain")
		c.IntOpt(&opts.Retries, "retries", "", 1, "Download request attempts per URL")
		c.IntOpt(&opts.ChunkConcurrency, "chunk", "", -1, "HTTP Range chunk concurrency: 0 auto, 1 single connection")
		c.IntOpt(&opts.BatchConcurrency, "batch", "", -1, "Concurrent package tasks for --all: 0 auto, 1 serial")
		c.AddArg("target", "Target(s) to update", false, true)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		targetArgs := append(c.Arg("target").Strings(), args...)
		if err := validateNoFlagArgs(targetArgs); err != nil {
			return err
		}
		if err := validateRetries(opts.Retries); err != nil {
			return err
		}
		opts.Targets = splitTargets(targetArgs)
		snapshot := *opts
		snapshot.Targets = append([]string(nil), opts.Targets...)
		return handler("update", &snapshot)
	}
	return cmd, func() {
		*opts = UpdateOptions{Retries: 1, ChunkConcurrency: -1, BatchConcurrency: -1}
	}
}
