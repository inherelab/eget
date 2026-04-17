package cli

import "github.com/gookit/goutil/cflag/capp"

type UpdateOptions struct {
	Target string
}

func newUpdateCmd(r *runner) *capp.Cmd {
	opts := &UpdateOptions{}
	cmd := capp.NewCmd("update", "Update installed targets", func(cmd *capp.Cmd) error {
		if cmd.Arg("target") != nil {
			opts.Target = cmd.Arg("target").String()
		}
		if err := validateNoTrailingFlags(cmd); err != nil {
			return err
		}
		if r != nil {
			r.result.Command = cmd.Name
			r.result.Options = opts
		}
		return ErrNotImplemented
	})

	cmd.AddArg("target", "Target to update", false, nil)
	return cmd
}
