package cli

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/gookit/goutil/cflag/capp"
)

var (
	ErrCommandRequired = errors.New("command required")
	ErrNotImplemented  = errors.New("not implemented")

	defaultHandlerOnce sync.Once
	defaultHandlerFn   CommandHandler
	defaultHandlerErr  error
)

type CommandHandler func(name string, options any) error

type App struct {
	inner     *capp.App
	resetters []func()
}

func Main(args []string, stdout, stderr io.Writer) error {
	app := newApp(defaultCommandHandler, stdout, stderr)
	err := app.RunWithArgs(args)
	if len(args) == 0 {
		return ErrCommandRequired
	}
	return err
}

func newApp(handler CommandHandler, stdout, stderr io.Writer) *App {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if handler == nil {
		handler = defaultCommandHandler
	}

	inner := capp.NewApp()
	inner.Name = "eget"
	inner.Desc = "Easy install and download tool"
	inner.HelpWriter = stdout
	inner.SetOutput(stderr)

	app := &App{inner: inner}
	app.add(newInstallCmd(handler))
	app.add(newDownloadCmd(handler))
	app.add(newAddCmd(handler))
	app.add(newUpdateCmd(handler))
	app.add(newConfigCmd(handler))
	return app
}

func (a *App) add(cmd *capp.Cmd, reset func()) {
	a.inner.Add(cmd)
	a.resetters = append(a.resetters, reset)
}

func (a *App) RunWithArgs(args []string) error {
	for _, reset := range a.resetters {
		reset()
	}
	return a.inner.RunWithArgs(args)
}

func defaultCommandHandler(name string, options any) error {
	defaultHandlerOnce.Do(func() {
		service, err := newCLIService()
		if err != nil {
			defaultHandlerErr = err
			return
		}
		defaultHandlerFn = service.handle
	})
	if defaultHandlerErr != nil {
		return defaultHandlerErr
	}
	if defaultHandlerFn == nil {
		return ErrNotImplemented
	}
	return defaultHandlerFn(name, options)
}

func validateNoTrailingFlags(cmd *capp.Cmd) error {
	for _, arg := range cmd.RemainArgs() {
		if len(arg) > 0 && arg[0] == '-' {
			return fmt.Errorf("flags must appear before arguments: %s", arg)
		}
	}
	return nil
}
