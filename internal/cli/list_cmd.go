package cli

import "github.com/gookit/goutil/cflag/capp"

type ListOptions struct{}

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
	return cmd, func() {
		*opts = ListOptions{}
	}
}
