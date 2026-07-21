package cli

import "github.com/gookit/gcli/v3"

type DownloadOptions struct {
	Tag              string
	System           string
	To               string
	File             string
	Asset            string
	Rename           string
	StripComponents  int
	Source           bool
	Prerelease       bool
	All              bool
	Quiet            bool
	Ghproxy          bool
	FallbackVersions int
	Retries          int
	ChunkConcurrency int
	Target           string
}

func newDownloadCmd(handler CommandHandler) (*gcli.Command, func()) {
	opts := &DownloadOptions{Retries: 1, ChunkConcurrency: -1}
	cmd := gcli.NewCommand("download", "Download a target")
	cmd.Aliases = []string{"dl"}
	cmd.Config = func(c *gcli.Command) {
		c.StrOpt(&opts.Tag, "tag", "", "", "Release tag")
		c.StrOpt(&opts.System, "system", "", "", "Target system. eg: linux/amd64")
		c.StrOpt(&opts.To, "to", "", "", "Download destination")
		c.StrOpt(&opts.File, "file", "", "", "File to extract, multi use comma split, support glob")
		c.StrOpt(&opts.Asset, "asset", "a", "", "Asset filter, multi use comma split")
		c.StrOpt(&opts.Rename, "rename", "", "", "Rename extracted files, comma separated from=to pairs")
		c.IntOpt(&opts.StripComponents, "strip-components", "", 0, "Strip leading archive path components when extracting all files")
		c.BoolOpt(&opts.Source, "source", "", false, "Download source archive")
		c.BoolOpt(&opts.Prerelease, "prerelease", "p", false, "Select latest release including prereleases")
		c.BoolOpt(&opts.All, "extract-all", "ea", false, "Extract all files")
		c.BoolOpt(&opts.Quiet, "quiet", "", false, "Quiet output; select first asset when multiple candidates remain")
		c.BoolOpt(&opts.Ghproxy, "ghproxy", "", false, "Rewrite GitHub download URL with configured ghproxy")
		c.IntOpt(&opts.FallbackVersions, "fallback-versions", "", 0, "Search older SourceForge version folders when asset is missing")
		c.IntOpt(&opts.Retries, "retries", "", 1, "Download request attempts per URL")
		c.IntOpt(&opts.ChunkConcurrency, "chunk", "", -1, "HTTP Range chunk concurrency: 0 auto, 1 single connection")
		c.AddArg("target", "Download target", true)
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
		return handler("download", &snapshot)
	}
	return cmd, func() {
		*opts = DownloadOptions{Retries: 1, ChunkConcurrency: -1}
	}
}
