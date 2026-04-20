package cli

import "github.com/gookit/goutil/cflag/capp"

type UninstallOptions struct {
	Target string
}

func newUninstallCmd(handler CommandHandler) (*capp.Cmd, func()) {
	opts := &UninstallOptions{}
	cmd := capp.NewCmd("uninstall", "Uninstall a managed package or repo", func(cmd *capp.Cmd) error {
		opts.Target = cmd.Arg("target").String()
		if err := validateNoTrailingFlags(cmd); err != nil {
			return err
		}
		snapshot := *opts
		return handler(cmd.Name, &snapshot)
	})

	cmd.Aliases = []string{"uni", "remove", "rm"}
	cmd.AddArg("target", "Package name or repo to uninstall", true, nil)
	return cmd, func() {
		*opts = UninstallOptions{}
	}
}
