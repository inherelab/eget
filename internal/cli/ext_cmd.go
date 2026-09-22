package cli

import "github.com/gookit/gcli/v3"

// ExtOptions holds the options of the `eget ext` subcommands.
type ExtOptions struct {
	Targets []string
}

func newExtCmd(handler CommandHandler) (*gcli.Command, func()) {
	listOpts := &ExtOptions{}
	upgradeOpts := &ExtOptions{}

	cmd := gcli.NewCommand("ext", "Manage packages installed by external package managers")
	cmd.Aliases = []string{"external"}
	cmd.Subs = []*gcli.Command{
		newExtListCmd(listOpts, handler),
		newExtUpgradeCmd(upgradeOpts, handler),
	}
	reset := func() {
		*listOpts = ExtOptions{}
		*upgradeOpts = ExtOptions{}
	}
	return cmd, reset
}

func newExtListCmd(opts *ExtOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("list", "List configured external managers and what they own")
	cmd.Aliases = []string{"ls"}
	cmd.Func = func(_ *gcli.Command, args []string) error {
		if err := validateNoFlagArgs(args); err != nil {
			return err
		}
		snapshot := *opts
		return handler("ext.list", &snapshot)
	}
	return cmd
}

func newExtUpgradeCmd(opts *ExtOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("upgrade", "Upgrade every package of one external manager")
	cmd.Config = func(c *gcli.Command) {
		c.AddArg("name", "Manager name, followed by optional package names", true, true)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		values := append(c.Arg("name").Strings(), args...)
		if err := validateNoFlagArgs(values); err != nil {
			return err
		}
		snapshot := ExtOptions{Targets: splitTargets(values)}
		return handler("ext.upgrade", &snapshot)
	}
	return cmd
}
