package cli

import "github.com/gookit/goutil/cflag/capp"

type DownloadOptions struct {
	Target string
}

func newDownloadCmd(r *runner) *capp.Cmd {
	opts := &DownloadOptions{}
	cmd := capp.NewCmd("download", "Download a target", func(cmd *capp.Cmd) error {
		opts.Target = cmd.Arg("target").String()
		if err := validateNoTrailingFlags(cmd); err != nil {
			return err
		}
		if r != nil {
			r.result.Command = cmd.Name
			r.result.Options = opts
		}
		return ErrNotImplemented
	})

	cmd.AddArg("target", "Download target", true, nil)
	return cmd
}
