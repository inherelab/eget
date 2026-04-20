package cli

import "github.com/gookit/goutil/cflag/capp"

type ConfigOptions struct {
	Info   bool
	Init   bool
	List   bool
	Action string
	Key    string
	Value  string
}

func newConfigCmd(handler CommandHandler) (*capp.Cmd, func()) {
	opts := &ConfigOptions{}
	cmd := capp.NewCmd("config", "Manage configuration", func(cmd *capp.Cmd) error {
		opts.Action = cmd.Arg("action").String()
		opts.Key = cmd.Arg("key").String()
		opts.Value = cmd.Arg("value").String()

		if err := validateNoTrailingFlags(cmd); err != nil {
			return err
		}
		snapshot := *opts
		return handler(cmd.Name, &snapshot)
	})

	cmd.Aliases = []string{"cfg"}

	cmd.BoolVar(&opts.Info, "info", false, "Show config information")
	cmd.BoolVar(&opts.Init, "init", false, "Initialize config file")
	cmd.BoolVar(&opts.List, "list", false, "List config values")
	cmd.AddArg("action", "Config action, allowed: get, set", false, nil)
	cmd.AddArg("key", "Config key", false, nil)
	cmd.AddArg("value", "Config value for set", false, nil)
	return cmd, func() {
		*opts = ConfigOptions{}
	}
}
