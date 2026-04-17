package cli

import "github.com/gookit/goutil/cflag/capp"

type ConfigOptions struct {
	Info bool
}

func newConfigCmd(handler CommandHandler) *capp.Cmd {
	opts := &ConfigOptions{}
	cmd := capp.NewCmd("config", "Manage configuration", func(cmd *capp.Cmd) error {
		if err := validateNoTrailingFlags(cmd); err != nil {
			return err
		}
		return handler(cmd.Name, opts)
	})

	cmd.BoolVar(&opts.Info, "info", false, "Show config information")
	return cmd
}
