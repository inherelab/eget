package cli

import "github.com/gookit/goutil/cflag/capp"

type InstallOptions struct {
	Tag    string
	System string
	To     string
	File   string
	Asset  string
	Source bool
	All    bool
	Quiet  bool
	Target string
}

func newInstallCmd(handler CommandHandler) (*capp.Cmd, func()) {
	opts := &InstallOptions{}
	cmd := capp.NewCmd("install", "Install a target", func(cmd *capp.Cmd) error {
		opts.Target = cmd.Arg("target").String()
		if err := validateNoTrailingFlags(cmd); err != nil {
			return err
		}
		snapshot := *opts
		return handler(cmd.Name, &snapshot)
	})
	cmd.Aliases = []string{"ins"}

	cmd.StringVar(&opts.Tag, "tag", "", "Release tag")
	cmd.StringVar(&opts.System, "system", "", "Target system")
	cmd.StringVar(&opts.To, "to", "", "Install destination")
	cmd.StringVar(&opts.File, "file", "", "File to extract")
	cmd.StringVar(&opts.Asset, "asset", "", "Asset filter")
	cmd.BoolVar(&opts.Source, "source", false, "Download source archive")
	cmd.BoolVar(&opts.All, "all", false, "Extract all files")
	cmd.BoolVar(&opts.Quiet, "quiet", false, "Quiet output")
	cmd.AddArg("target", "Installation target", true, nil)
	return cmd, func() {
		*opts = InstallOptions{}
	}
}
