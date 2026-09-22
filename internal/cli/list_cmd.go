package cli

import "github.com/gookit/gcli/v3"

type ListOptions struct {
	Outdated    bool
	All         bool
	GUI         bool
	NoInstalled bool
	Info        string
	Ext         string
	WithExt     string
}

func newListCmd(handler CommandHandler) (*gcli.Command, func()) {
	opts := &ListOptions{}
	cmd := gcli.NewCommand("list", "List managed packages")
	cmd.Aliases = []string{"ls"}
	cmd.Config = func(c *gcli.Command) {
		c.BoolOpt(&opts.Outdated, "outdated", "old", false, "Check and list outdated installed packages")
		c.BoolOpt(&opts.All, "all", "a", false, "List all managed and installed packages")
		c.BoolOpt(&opts.GUI, "gui", "", false, "List GUI applications")
		c.BoolOpt(&opts.NoInstalled, "no-installed", "ni", false, "List packages configured but not installed")
		c.StrOpt(&opts.Info, "info", "i", "", "Show detailed info for a package")
		c.StrOpt(&opts.Ext, "ext", "", "", "Only list packages of these external managers: all or npm,bun")
		c.StrOpt(&opts.WithExt, "with-ext", "", "", "Also list packages of these external managers: all or npm,bun")
	}
	cmd.Func = func(_ *gcli.Command, args []string) error {
		if err := validateNoFlagArgs(args); err != nil {
			return err
		}
		snapshot := *opts
		return handler("list", &snapshot)
	}
	return cmd, func() {
		*opts = ListOptions{}
	}
}
