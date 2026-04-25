package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/gookit/goutil/cflag/capp"
)

var (
	version   string
	gitHash   string
	buildTime string
)

var (
	ErrNotImplemented = errors.New("not implemented")
)

type CommandHandler func(name string, options any) error

type App struct {
	inner     *capp.App
	resetters []func()
	verbose   *bool
}

// SetBuildInfo sets the build information for the application.
func SetBuildInfo(versionStr, gitHashStr, buildTimeStr string) {
	version = versionStr
	gitHash = gitHashStr
	buildTime = buildTimeStr
}

func Main(args []string, stdout, stderr io.Writer) error {
	var service *cliService
	var serviceErr error
	var app *App
	app = newApp(func(name string, options any) error {
		if service == nil && serviceErr == nil {
			service, serviceErr = newCLIService()
		}
		if serviceErr != nil {
			return serviceErr
		}
		if service == nil {
			return ErrNotImplemented
		}
		configureVerbose(app.Verbose(), stderr)
		return service.handle(name, options)
	}, stdout, stderr)
	return app.RunWithArgs(args)
}

func newApp(handler CommandHandler, stdout, stderr io.Writer) *App {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if handler == nil {
		handler = func(name string, options any) error {
			_ = name
			_ = options
			return ErrNotImplemented
		}
	}

	inner := capp.NewApp()
	inner.Name = "eget"
	inner.Desc = fmt.Sprintf(
		"Easy install and download tools from GitHub\n  (%s, %s)",
		gitHash, buildTime,
	)
	inner.Version = version
	inner.HelpWriter = stdout
	inner.SetOutput(stderr)
	verbose := false
	inner.BoolVar(&verbose, "verbose", false, "Show verbose execution details")
	inner.AddShortcuts("verbose", "v")

	app := &App{inner: inner, verbose: &verbose}
	app.add(newInstallCmd(handler))
	app.add(newDownloadCmd(handler))
	app.add(newAddCmd(handler))
	app.add(newUninstallCmd(handler))
	app.add(newListCmd(handler))
	app.add(newUpdateCmd(handler))
	app.add(newConfigCmd(handler))
	app.add(newQueryCmd(handler))
	app.add(newSearchCmd(handler))
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
	if a.verbose != nil {
		*a.verbose = false
	}
	return a.inner.RunWithArgs(args)
}

func (a *App) Verbose() bool {
	return a.verbose != nil && *a.verbose
}

func validateNoTrailingFlags(cmd *capp.Cmd) error {
	for _, arg := range cmd.RemainArgs() {
		if len(arg) > 0 && arg[0] == '-' {
			return fmt.Errorf("flags must appear before arguments: %s", arg)
		}
	}
	return nil
}
