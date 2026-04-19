package cli

import "github.com/gookit/goutil/cflag/capp"

type AddOptions struct {
	Name     string
	Tag      string
	System   string
	To       string
	CacheDir string
	File     string
	Asset    string
	Source   bool
	All      bool
	Quiet    bool
	Target   string
}

func newAddCmd(handler CommandHandler) (*capp.Cmd, func()) {
	opts := &AddOptions{}
	cmd := capp.NewCmd("add", "Add a managed package", func(cmd *capp.Cmd) error {
		opts.Target = cmd.Arg("target").String()
		if err := validateNoTrailingFlags(cmd); err != nil {
			return err
		}
		snapshot := *opts
		return handler(cmd.Name, &snapshot)
	})

	cmd.StringVar(&opts.Name, "name", "", "Managed package name")
	cmd.StringVar(&opts.Tag, "tag", "", "Release tag")
	cmd.StringVar(&opts.System, "system", "", "Target system")
	cmd.StringVar(&opts.To, "to", "", "Install destination")
	cmd.StringVar(&opts.CacheDir, "cache-dir", "", "Download cache directory")
	cmd.StringVar(&opts.File, "file", "", "File to extract")
	cmd.StringVar(&opts.Asset, "asset", "", "Asset filter")
	cmd.BoolVar(&opts.Source, "source", false, "Download source archive")
	cmd.BoolVar(&opts.All, "all", false, "Extract all files")
	cmd.BoolVar(&opts.Quiet, "quiet", false, "Quiet output")
	cmd.AddArg("target", "Package target", true, nil)
	return cmd, func() {
		*opts = AddOptions{}
	}
}
