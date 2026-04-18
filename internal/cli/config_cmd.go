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
		args := cmd.Args()
		if len(args) > 0 {
			opts.Action = cmd.Arg("action").String()
		}
		if len(args) > 1 {
			opts.Key = cmd.Arg("key").String()
		}
		if len(args) > 2 {
			opts.Value = cmd.Arg("value").String()
		}
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
	cmd.AddArg("action", "Config action", false, nil)
	cmd.AddArg("key", "Config key", false, nil)
	cmd.AddArg("value", "Config value", false, nil)
	return cmd, func() {
		*opts = ConfigOptions{}
	}
}
