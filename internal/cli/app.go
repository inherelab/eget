package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gookit/color"
	"github.com/gookit/gcli/v3"
	clirender "github.com/inherelab/eget/internal/cli/render"
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

type Info struct {
	Version   string
	GitHash   string
	BuildTime string
}

type App struct {
	inner     *gcli.App
	commands  []*gcli.Command
	resetters []func()
	verbose   *bool
	noProxy   *bool
	lastErr   error
	stdout    io.Writer
}

// SetBuildInfo sets the build information for the application.
func SetBuildInfo(versionStr, gitHashStr, buildTimeStr string) {
	version = versionStr
	gitHash = gitHashStr
	buildTime = normalizeBuildTime(buildTimeStr)
}

func BuildInfo() Info {
	return Info{
		Version:   version,
		GitHash:   gitHash,
		BuildTime: buildTime,
	}
}

func normalizeBuildTime(value string) string {
	for _, layout := range []string{
		clirender.CompactTimeLayout,
		time.RFC3339,
		"2006/01/02-15:04:05",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Format(clirender.CompactTimeLayout)
		}
	}
	return value
}

var cliApp *App

func Main(args []string, stdout, stderr io.Writer) error {
	var service *cliService
	var serviceErr error
	cliApp = newApp(func(name string, options any) error {
		if service == nil && serviceErr == nil {
			service, serviceErr = newCLIService(cliApp.NoProxy())
		}
		if serviceErr != nil {
			return serviceErr
		}
		if service == nil {
			return ErrNotImplemented
		}
		service.stderr = stderr
		configureVerbose(cliApp.Verbose(), stderr)
		return service.handle(name, options)
	}, stdout, stderr)
	return cliApp.RunWithArgs(args)
}

func newApp(handler CommandHandler, stdout, stderr io.Writer) *App {
	if stdout == nil {
		stdout = io.Discard
	}
	if handler == nil {
		handler = func(name string, options any) error {
			_ = name
			_ = options
			return ErrNotImplemented
		}
	}

	inner := gcli.NewApp(gcli.NotExitOnEnd())
	inner.Name = "eget"
	inner.Desc = "Easy install and download tools from GitHub, SourceForge and more"
	inner.Version = buildVersionString()
	verbose := false
	noProxy := false
	app := &App{inner: inner, verbose: &verbose, noProxy: &noProxy, stdout: stdout}
	cliApp = app
	inner.Flags().BoolOpt(app.verbose, "verbose", "v", false, "Show detailed debug information")
	inner.Flags().BoolOpt(app.noProxy, "no-proxy", "", false, "Disable http proxy for request")
	inner.On(gcli.EvtAppRunError, func(ctx *gcli.HookCtx) bool {
		if errV := ctx.Get("err"); errV != nil {
			if err, ok := errV.(error); ok {
				app.lastErr = err
			}
		}
		return true
	})
	app.add(newInstallCmd(handler))
	app.add(newDownloadCmd(handler))
	app.add(newSDKCmd(handler))
	app.add(newCacheCmd(handler))
	app.add(newAddCmd(handler))
	app.add(newUninstallCmd(handler))
	app.add(newListCmd(handler))
	app.add(newShowCmd(handler))
	app.add(newUpdateCmd(handler))
	app.add(newConfigCmd(handler))
	app.add(newQueryCmd(handler))
	app.add(newSearchCmd(handler))
	return app
}

func (a *App) add(cmd *gcli.Command, reset func()) {
	if cmd.Help == "" {
		cmd.Help = commonCommandHelp(cmd.Name)
	}
	a.inner.Add(cmd)
	a.commands = append(a.commands, cmd)
	a.resetters = append(a.resetters, reset)
}

func commonCommandHelp(name string) string {
	switch name {
	case "install":
		return `<info>Examples</>:
  eget install sharkdp/fd
  eget install t8y2/dbx --gui --install-mode installer
  eget install iOfficeAI/AionUi --asset "x86_64,linux,gz" --extract-all --strip-components 1 --to ~/Downloads/AionUi
  eget install inhere/markview --add --name markview`
	case "download":
		return `<info>Examples</>:
  eget download sharkdp/fd
  eget download gookit/gitw --tag v0.3.6 --asset "linux,amd64,tar.gz"
  eget download sourceforge:keepass/Translations --fallback-versions 10 --asset German.zip --to ./downloads`
	case "add":
		return `<info>Examples</>:
  eget add sharkdp/fd
  eget add t8y2/dbx --name dbx --gui
  eget add iOfficeAI/AionUi --asset "x86_64,linux,gz" --extract-all --strip-components 1 --to ~/Downloads/AionUi`
	case "update":
		return `<info>Examples</>:
  eget update sshc
  eget update --check
  eget update --all
  eget update --all --dry-run
  eget update --self`
	case "list":
		return `<info>Examples</>:
  eget list
  eget list --all
  eget list --outdated
  eget list --gui
  eget list --info sshc`
	case "show":
		return `<info>Examples</>:
  eget show sshc
  eget show sharkdp/fd`
	case "uninstall":
		return `<info>Examples</>:
  eget uninstall sshc markview
  eget uninstall sshc --yes
  eget uninstall sshc --purge`
	default:
		return ""
	}
}

func (a *App) RunWithArgs(args []string) error {
	a.lastErr = nil
	if a.verbose != nil {
		*a.verbose = false
	}
	if a.noProxy != nil {
		*a.noProxy = false
	}
	for _, reset := range a.resetters {
		reset()
	}
	for _, cmd := range a.commands {
		for _, arg := range cmd.Args() {
			arg.Reset()
		}
	}
	if err := validateKnownFlags(args); err != nil {
		return err
	}
	color.SetOutput(a.stdout)
	defer color.ResetOutput()
	a.inner.Run(args)
	return a.lastErr
}

func (a *App) Verbose() bool {
	return a.verbose != nil && *a.verbose
}

func (a *App) NoProxy() bool {
	return a.noProxy != nil && *a.noProxy
}

func buildVersionString() string {
	if gitHash == "" && buildTime == "" {
		return version
	}
	return fmt.Sprintf("%s (%s, %s)", version, gitHash, buildTime)
}

func validateNoFlagArgs(args []string) error {
	for _, arg := range args {
		if len(arg) > 0 && arg[0] == '-' {
			return fmt.Errorf("flags must appear before arguments: %s", arg)
		}
	}
	return nil
}

type flagSpec struct {
	bools  map[string]bool
	values map[string]bool
	subs   map[string]flagSpec
}

var commandAliases = map[string]string{
	"i":   "install",
	"ins": "install",
	"dl":  "download",
	"uni": "uninstall",
	"rm":  "uninstall",
	"ls":  "list",
	"up":  "update",
	"cfg": "config",
	"q":   "query",
}

var commandFlagSpecs = map[string]flagSpec{
	"install": {
		bools:  setOf("source", "prerelease", "p", "extract-all", "ea", "all", "gui", "quiet", "add"),
		values: setOf("tag", "system", "to", "file", "asset", "a", "rename", "name", "install-mode", "strip-components", "fallback-versions", "chunk", "batch"),
	},
	"download": {
		bools:  setOf("source", "prerelease", "p", "extract-all", "ea", "quiet", "ghproxy"),
		values: setOf("tag", "system", "to", "file", "asset", "a", "rename", "strip-components", "fallback-versions", "chunk"),
	},
	"add": {
		bools:  setOf("source", "extract-all", "ea", "gui", "quiet"),
		values: setOf("name", "tag", "system", "to", "file", "asset", "rename", "strip-components", "chunk"),
	},
	"list": {
		bools:  setOf("outdated", "old", "all", "a", "gui", "no-installed", "ni"),
		values: setOf("info", "i"),
	},
	"update": {
		bools:  setOf("all", "A", "check", "dry-run", "interactive", "i", "self", "source", "quiet"),
		values: setOf("self-source", "tag", "system", "to", "file", "asset", "a", "chunk", "batch"),
	},
	"query": {
		bools:  setOf("json", "j", "prerelease", "p"),
		values: setOf("action", "a", "tag", "t", "limit", "l"),
	},
	"sdk": {
		subs: map[string]flagSpec{
			"download": {values: setOf("os", "arch", "output", "o")},
			"dl":       {values: setOf("os", "arch", "output", "o")},
			"install":  {bools: setOf("force", "f")},
			"i":        {bools: setOf("force", "f")},
			"ins":      {bools: setOf("force", "f")},
			"list":     {bools: setOf("json", "j")},
			"ls":       {bools: setOf("json", "j")},
			"remove":   {},
			"rm":       {},
			"path":     {},
			"search":   {bools: setOf("json", "j"), values: setOf("number", "n", "sort")},
			"config": {
				subs: map[string]flagSpec{
					"add": {bools: setOf("all", "a", "force", "f"), values: setOf("mirror", "m")},
				},
			},
			"cfg": {
				subs: map[string]flagSpec{
					"add": {bools: setOf("all", "a", "force", "f"), values: setOf("mirror", "m")},
				},
			},
			"index": {
				subs: map[string]flagSpec{
					"list":    {bools: setOf("json", "j")},
					"ls":      {bools: setOf("json", "j")},
					"show":    {},
					"refresh": {bools: setOf("all", "a")},
					"build":   {bools: setOf("all", "a")},
					"clear":   {bools: setOf("all", "a")},
				},
			},
			"idx": {
				subs: map[string]flagSpec{
					"list":    {bools: setOf("json", "j")},
					"ls":      {bools: setOf("json", "j")},
					"show":    {},
					"refresh": {bools: setOf("all", "a")},
					"build":   {bools: setOf("all", "a")},
					"clear":   {bools: setOf("all", "a")},
				},
			},
		},
	},
	"cache": {
		subs: map[string]flagSpec{
			"list":   {bools: setOf("json", "j"), values: setOf("root")},
			"status": {bools: setOf("json", "j")},
			"clean":  {bools: setOf("all", "a", "dry-run", "yes", "y", "pkg", "api", "sdk", "sdk-index", "partial", "json", "j"), values: setOf("older")},
			"serve":  {bools: setOf("no-index", "json-log"), values: setOf("host", "port", "p", "root", "token")},
		},
	},
	"search":    {bools: setOf("json", "j"), values: setOf("sort", "order", "limit", "l")},
	"show":      {},
	"uninstall": {bools: setOf("yes", "y", "purge")},
	"config": {
		subs: map[string]flagSpec{
			"init":   {},
			"list":   {},
			"ls":     {},
			"doctor": {},
			"path":   {bools: setOf("check")},
			"get":    {},
			"set":    {},
		},
	},
}

func setOf(names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

func validateKnownFlags(args []string) error {
	spec, start := findCommandSpec(args)
	if start < 0 {
		return nil
	}
	for i := start; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return nil
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			continue
		}
		name := strings.TrimLeft(arg, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		if name == "help" || name == "h" {
			continue
		}
		if spec.bools[name] {
			continue
		}
		if spec.values[name] {
			if strings.Contains(arg, "=") {
				if (name == "mirror" || name == "m") && strings.HasSuffix(arg, "=") {
					return fmt.Errorf("%s requires a value", strings.SplitN(arg, "=", 2)[0])
				}
			} else {
				if (name == "mirror" || name == "m") && (i+1 >= len(args) || strings.HasPrefix(args[i+1], "-")) {
					return fmt.Errorf("%s requires a value", arg)
				}
				i++
			}
			continue
		}
		return fmt.Errorf("option provided but not defined: %s", arg)
	}
	return nil
}

func findCommandSpec(args []string) (flagSpec, int) {
	cmdName, start := findCommandArg(args)
	if cmdName == "" {
		return flagSpec{}, -1
	}
	spec, ok := commandFlagSpecs[cmdName]
	if !ok {
		return flagSpec{}, -1
	}
	for len(spec.subs) > 0 && start < len(args) {
		subName := args[start]
		if strings.HasPrefix(subName, "-") {
			break
		}
		subSpec, ok := spec.subs[subName]
		if !ok {
			break
		}
		spec = subSpec
		start++
	}
	return spec, start
}

func findCommandArg(args []string) (string, int) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-v", "--verbose", "--no-proxy", "-V", "--version", "-h", "--help":
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if real, ok := commandAliases[arg]; ok {
			arg = real
		}
		return arg, i + 1
	}
	return "", 0
}
