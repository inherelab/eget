package cli

import "github.com/gookit/goutil/cflag/capp"

type DownloadOptions struct {
	Tag      string
	System   string
	To       string
	File     string
	Asset    string
	Source   bool
	All      bool
	Quiet    bool
	Target   string
}

func newDownloadCmd(handler CommandHandler) (*capp.Cmd, func()) {
	opts := &DownloadOptions{}
	cmd := capp.NewCmd("download", "Download a target", func(cmd *capp.Cmd) error {
		opts.Target = cmd.Arg("target").String()
		if err := validateNoTrailingFlags(cmd); err != nil {
			return err
		}
		snapshot := *opts
		return handler(cmd.Name, &snapshot)
	})
	cmd.Aliases = []string{"dl"}

	cmd.StringVar(&opts.Tag, "tag", "", "Release tag")
	cmd.StringVar(&opts.System, "system", "", "Target system")
	cmd.StringVar(&opts.To, "to", "", "Download destination")
	cmd.StringVar(&opts.File, "file", "", "File to extract, multi use comma split, support glob")
	cmd.StringVar(&opts.Asset, "asset", "", "Asset filter, multi use comma split;;a")
	cmd.BoolVar(&opts.Source, "source", false, "Download source archive")
	cmd.BoolVar(&opts.All, "extract-all", false, "Extract all files;;ea")
	cmd.BoolVar(&opts.Quiet, "quiet", false, "Quiet output")
	cmd.AddArg("target", "Download target", true, nil)
	return cmd, func() {
		*opts = DownloadOptions{}
	}
}
