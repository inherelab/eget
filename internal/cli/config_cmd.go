package cli

import "github.com/gookit/goutil/cflag/capp"

type ConfigOptions struct {
	Info bool
}

func newConfigCmd(r *runner) *capp.Cmd {
	opts := &ConfigOptions{}
	cmd := capp.NewCmd("config", "Manage configuration", func(cmd *capp.Cmd) error {
		if err := validateNoTrailingFlags(cmd); err != nil {
			return err
		}
		if r != nil {
			r.result.Command = cmd.Name
			r.result.Options = opts
		}
		return ErrNotImplemented
	})

	cmd.BoolVar(&opts.Info, "info", false, "Show config information")
	return cmd
}
