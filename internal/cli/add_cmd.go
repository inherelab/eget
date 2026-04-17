package cli

import "github.com/gookit/goutil/cflag/capp"

type AddOptions struct {
	Target string
}

func newAddCmd(r *runner) *capp.Cmd {
	opts := &AddOptions{}
	cmd := capp.NewCmd("add", "Add a managed package", func(cmd *capp.Cmd) error {
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

	cmd.AddArg("target", "Package target", true, nil)
	return cmd
}
