package cli

import "github.com/gookit/goutil/cflag/capp"

type DownloadOptions struct {
	Target string
}

func newDownloadCmd(handler CommandHandler) *capp.Cmd {
	opts := &DownloadOptions{}
	cmd := capp.NewCmd("download", "Download a target", func(cmd *capp.Cmd) error {
		opts.Target = cmd.Arg("target").String()
		if err := validateNoTrailingFlags(cmd); err != nil {
			return err
		}
		return handler(cmd.Name, opts)
	})

	cmd.AddArg("target", "Download target", true, nil)
	return cmd
}
