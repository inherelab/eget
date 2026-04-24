package cli

import "github.com/gookit/goutil/cflag/capp"

type ConfigOptions struct {
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
	cmd.LongHelp = `<info>Config actions</>:
  init                Initialize the config file with default values
  list | ls           Print current config values and file status
  get KEY             Print one config value
  set KEY VALUE       Update one config value

<info>Examples</>:
  eget config init
  eget config list
  eget config get global.target
  eget config set global.target ~/.local/bin`

    cmd.AddArg("action", "Config action: init, list, ls, get, set", false, nil)
	cmd.AddArg("key", "Config key", false, nil)
	cmd.AddArg("value", "Config value for set", false, nil)
	return cmd, func() {
		*opts = ConfigOptions{}
	}
}
