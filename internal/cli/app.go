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

type RunResult struct {
	Command string
	Options any
	Err     error
}

type runner struct {
	result RunResult
}

func Run(args []string) RunResult {
	r := &runner{}
	app := newApp(r)

	err := app.RunWithArgs(args)
	if err != nil {
		r.result.Err = err
	}

	if len(args) == 0 && r.result.Err == nil {
		r.result.Err = ErrCommandRequired
	}
	return r.result
}

func Main(args []string, stdout, stderr io.Writer) error {
	_ = stderr
	app := newApp(nil)
	app.HelpWriter = stdout
	return app.RunWithArgs(args)
}

func newApp(r *runner) *capp.App {
	app := capp.NewApp()
	app.Name = "eget"
	app.Desc = "Easy install and download tool"

	if r != nil {
		app.AfterRun = func(cmd *capp.Cmd, err error) {
			r.result.Command = cmd.Name
			r.result.Err = err
		}
	}

	app.Add(
		newInstallCmd(r),
		newDownloadCmd(r),
		newAddCmd(r),
		newUpdateCmd(r),
		newConfigCmd(r),
	)
	return app
}

func validateNoTrailingFlags(cmd *capp.Cmd) error {
	for _, arg := range cmd.RemainArgs() {
		if len(arg) > 0 && arg[0] == '-' {
			return fmt.Errorf("flags must appear before arguments: %s", arg)
		}
	}
	return nil
}
