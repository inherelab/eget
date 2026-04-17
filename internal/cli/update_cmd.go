package cli

import "github.com/gookit/goutil/cflag/capp"

type UpdateOptions struct {
	Target string
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

	cmd.AddArg("target", "Target to update", false, nil)
	return cmd, func() {
		*opts = UpdateOptions{}
	}
}
