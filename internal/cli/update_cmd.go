package cli

import "github.com/gookit/goutil/cflag/capp"

type UpdateOptions struct {
	All         bool
	DryRun      bool
	Interactive bool
	Tag         string
	System      string
	To          string
	File        string
	Asset       string
	Source      bool
	Quiet       bool
	Target      string
}

func newUpdateCmd(handler CommandHandler) (*capp.Cmd, func()) {
	opts := &UpdateOptions{}
	cmd := capp.NewCmd("update", "Update installed targets", func(cmd *capp.Cmd) error {
		if len(cmd.Args()) > 0 {
			opts.Target = cmd.Arg("target").String()
		} else {
			opts.Target = ""
		}
		if err := validateNoTrailingFlags(cmd); err != nil {
			return err
		}
		snapshot := *opts
		return handler(cmd.Name, &snapshot)
	})
	cmd.Aliases = []string{"up"}

	cmd.BoolVar(&opts.All, "all", false, "Update all managed packages")
	cmd.BoolVar(&opts.DryRun, "dry-run", false, "Preview updates without changes")
	cmd.BoolVar(&opts.Interactive, "interactive", false, "Interactively choose packages")
	cmd.StringVar(&opts.Tag, "tag", "", "Release tag")
	cmd.StringVar(&opts.System, "system", "", "Target system")
	cmd.StringVar(&opts.To, "to", "", "Install destination")
	cmd.StringVar(&opts.File, "file", "", "File to extract, multi use comma split, support glob")
	cmd.StringVar(&opts.Asset, "asset", "", "Asset filter, multi use comma split;;a")
	cmd.BoolVar(&opts.Source, "source", false, "Download source archive")
	cmd.BoolVar(&opts.Quiet, "quiet", false, "Quiet output")
	cmd.AddArg("target", "Target to update", false, nil)
	return cmd, func() {
		*opts = UpdateOptions{}
	}
}
