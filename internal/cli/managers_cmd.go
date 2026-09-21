package cli

import "github.com/gookit/gcli/v3"

// ManagersOptions holds the options of the `eget managers` subcommands.
type ManagersOptions struct {
	Targets []string
}

func newManagersCmd(handler CommandHandler) (*gcli.Command, func()) {
	listOpts := &ManagersOptions{}
	upgradeOpts := &ManagersOptions{}

	cmd := gcli.NewCommand("managers", "Manage packages installed by external package managers")
	cmd.Aliases = []string{"mgr"}
	cmd.Subs = []*gcli.Command{
		newManagersListCmd(listOpts, handler),
		newManagersUpgradeCmd(upgradeOpts, handler),
	}
	reset := func() {
		*listOpts = ManagersOptions{}
		*upgradeOpts = ManagersOptions{}
	}
	return cmd, reset
}

func newManagersListCmd(opts *ManagersOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("list", "List configured managers and what they own")
	cmd.Aliases = []string{"ls"}
	cmd.Func = func(_ *gcli.Command, args []string) error {
		if err := validateNoFlagArgs(args); err != nil {
			return err
		}
		snapshot := *opts
		return handler("managers.list", &snapshot)
	}
	return cmd
}

func newManagersUpgradeCmd(opts *ManagersOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("upgrade", "Upgrade every package of one manager")
	cmd.Config = func(c *gcli.Command) {
		c.AddArg("name", "Manager name, followed by optional package names", true, true)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		values := append(c.Arg("name").Strings(), args...)
		if err := validateNoFlagArgs(values); err != nil {
			return err
		}
		snapshot := ManagersOptions{Targets: splitTargets(values)}
		return handler("managers.upgrade", &snapshot)
	}
	return cmd
}
