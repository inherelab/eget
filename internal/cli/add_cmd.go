package cli

import "github.com/gookit/goutil/cflag/capp"

type AddOptions struct {
	Target string
}

func newAddCmd(handler CommandHandler) *capp.Cmd {
	opts := &AddOptions{}
	cmd := capp.NewCmd("add", "Add a managed package", func(cmd *capp.Cmd) error {
		opts.Target = cmd.Arg("target").String()
		if err := validateNoTrailingFlags(cmd); err != nil {
			return err
		}
		return handler(cmd.Name, opts)
	})

	cmd.AddArg("target", "Package target", true, nil)
	return cmd
}
