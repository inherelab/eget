package cli

import "github.com/gookit/goutil/cflag/capp"

type ListOptions struct {
	Outdated bool
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
	return cmd, func() {
		*opts = ListOptions{}
	}
}
