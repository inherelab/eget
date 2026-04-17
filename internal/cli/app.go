package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/gookit/goutil/cflag/capp"
)

var (
	ErrCommandRequired = errors.New("command required")
	ErrNotImplemented  = errors.New("not implemented")
)

type CommandHandler func(name string, options any) error

func Main(args []string, stdout, stderr io.Writer) error {
	app := newApp(defaultCommandHandler, stdout, stderr)
	err := app.RunWithArgs(args)
	if len(args) == 0 {
		return ErrCommandRequired
	}
	return err
}

func newApp(handler CommandHandler, stdout, stderr io.Writer) *capp.App {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if handler == nil {
		handler = defaultCommandHandler
	}

	app := capp.NewApp()
	app.Name = "eget"
	app.Desc = "Easy install and download tool"
	app.HelpWriter = stdout
	app.SetOutput(stderr)
	app.Add(
		newInstallCmd(handler),
		newDownloadCmd(handler),
		newAddCmd(handler),
		newUpdateCmd(handler),
		newConfigCmd(handler),
	)
	return app
}

func defaultCommandHandler(name string, options any) error {
	_ = name
	_ = options
	return ErrNotImplemented
}

func validateNoTrailingFlags(cmd *capp.Cmd) error {
	for _, arg := range cmd.RemainArgs() {
		if len(arg) > 0 && arg[0] == '-' {
			return fmt.Errorf("flags must appear before arguments: %s", arg)
		}
	}
	return nil
}
