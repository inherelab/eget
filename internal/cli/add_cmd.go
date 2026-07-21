package cli

import "github.com/gookit/gcli/v3"

type AddOptions struct {
	Name             string
	Tag              string
	System           string
	To               string
	File             string
	Asset            string
	Rename           string
	StripComponents  int
	Source           bool
	All              bool
	GUI              bool
	Quiet            bool
	Retries          int
	ChunkConcurrency int
	Target           string
}

func newAddCmd(handler CommandHandler) (*gcli.Command, func()) {
	opts := &AddOptions{Retries: 1, ChunkConcurrency: -1}
	cmd := gcli.NewCommand("add", "Add a managed package")
	cmd.Config = func(c *gcli.Command) {
		c.StrOpt(&opts.Name, "name", "", "", "Managed package name")
		c.StrOpt(&opts.Tag, "tag", "", "", "Release tag")
		c.StrOpt(&opts.System, "system", "", "", "Target system. eg: linux/amd64")
		c.StrOpt(&opts.To, "to", "", "", "Install destination")
		c.StrOpt(&opts.File, "file", "", "", "File to extract, multi use comma split")
		c.StrOpt(&opts.Asset, "asset", "", "", "Asset filter, multi use comma split")
		c.StrOpt(&opts.Rename, "rename", "", "", "Rename extracted files, comma separated from=to pairs")
		c.IntOpt(&opts.StripComponents, "strip-components", "", 0, "Strip leading archive path components when extracting all files")
		c.BoolOpt(&opts.Source, "source", "", false, "Download source archive")
		c.BoolOpt(&opts.All, "extract-all", "ea", false, "Extract all files")
		c.BoolOpt(&opts.GUI, "gui", "", false, "Add as GUI application")
		c.BoolOpt(&opts.Quiet, "quiet", "", false, "Quiet output; select first asset when multiple candidates remain")
		c.IntOpt(&opts.Retries, "retries", "", 1, "Download request attempts per URL")
		c.IntOpt(&opts.ChunkConcurrency, "chunk", "", -1, "HTTP Range chunk concurrency: 0 auto, 1 single connection")
		c.AddArg("target", "Package target", true)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		opts.Target = c.Arg("target").String()
		if err := validateNoFlagArgs(append([]string{opts.Target}, args...)); err != nil {
			return err
		}
		if err := validateRetries(opts.Retries); err != nil {
			return err
		}
		snapshot := *opts
		return handler("add", &snapshot)
	}
	return cmd, func() {
		*opts = AddOptions{Retries: 1, ChunkConcurrency: -1}
	}
}
