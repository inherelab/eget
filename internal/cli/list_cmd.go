package cli

import "github.com/gookit/goutil/cflag/capp"

type ListOptions struct {
	Outdated bool
	All      bool
	GUI      bool
	Info     string
}

func newListCmd(handler CommandHandler) (*capp.Cmd, func()) {
	opts := &ListOptions{}
	cmd := capp.NewCmd("list", "List managed packages", func(cmd *capp.Cmd) error {
		if err := validateNoTrailingFlags(cmd); err != nil {
			return err
		}
		snapshot := *opts
		return handler(cmd.Name, &snapshot)
	})
	cmd.Aliases = []string{"ls"}
	cmd.BoolVar(&opts.Outdated, "outdated", false, "Check and list outdated installed packages")
	cmd.BoolVar(&opts.All, "all", false, "List all managed and installed packages;false;a")
	cmd.BoolVar(&opts.GUI, "gui", false, "List GUI applications")
	cmd.StringVar(&opts.Info, "info", "", "Show detailed info for a package;;i")
	return cmd, func() {
		*opts = ListOptions{}
	}
}
