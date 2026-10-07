package cli

import (
	"fmt"
	"strings"

	"github.com/gookit/gcli/v3"
)

type SDKInstallOptions struct {
	Targets []string
	Force   bool
}

type SDKDownloadOptions struct {
	Targets []string
	OS      string
	Arch    string
	Output  string
}

type SDKListOptions struct {
	Name string
	JSON bool
}

type SDKRemoveOptions struct {
	Target string
}

type SDKPathOptions struct {
	Target string
}

type SDKSearchOptions struct {
	Name       string
	Keywords   []string
	JSON       bool
	Number     int
	Sort       string
	Prerelease bool
}

type SDKIndexOptions struct {
	Action string
	Name   string
	All    bool
	JSON   bool
}

type SDKConfigOptions struct {
	Action string
	Name   string
	All    bool
	Force  bool
	Mirror string
}

func newSDKCmd(handler CommandHandler) (*gcli.Command, func()) {
	installOpts := &SDKInstallOptions{}
	downloadOpts := &SDKDownloadOptions{}
	listOpts := &SDKListOptions{}
	removeOpts := &SDKRemoveOptions{}
	pathOpts := &SDKPathOptions{}
	searchOpts := &SDKSearchOptions{Number: 20}
	indexOpts := &SDKIndexOptions{}
	configOpts := &SDKConfigOptions{}
	cmd := gcli.NewCommand("sdk", "Download and manage SDK archives")
	cmd.Help = `<info>Examples</>:
  eget sdk install go@1.22.0
  eget sdk install --force go:1.22 node:20.11.1
  eget sdk download go:1.22
  eget sdk dl --os linux --arch arm64 -o ./downloads go:1.22
  eget sdk list
  eget sdk remove go@1.22.0
  eget sdk path go
  eget sdk path java:17
  eget sdk search go 1.22 amd64 ^windows
  eget sdk search --sort desc node REG:^22
  eget sdk search --prerelease go 1.22
  eget sdk config add jdk --mirror huawei
  eget sdk config add jdk --mirror zulu
  eget sdk config add --all
	eget sdk index refresh go`
	cmd.Subs = []*gcli.Command{
		newSDKInstallCmd(installOpts, handler),
		newSDKDownloadCmd(downloadOpts, handler),
		newSDKListCmd(listOpts, handler),
		newSDKRemoveCmd(removeOpts, handler),
		newSDKPathCmd(pathOpts, handler),
		newSDKSearchCmd(searchOpts, handler),
		newSDKConfigCmd(configOpts, handler),
		newSDKIndexCmd(indexOpts, handler),
	}
	return cmd, func() {
		*installOpts = SDKInstallOptions{}
		*downloadOpts = SDKDownloadOptions{}
		*listOpts = SDKListOptions{}
		*removeOpts = SDKRemoveOptions{}
		*pathOpts = SDKPathOptions{}
		*searchOpts = SDKSearchOptions{Number: 20}
		*indexOpts = SDKIndexOptions{}
		*configOpts = SDKConfigOptions{}
	}
}

func newSDKDownloadCmd(opts *SDKDownloadOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("download", "Download SDK archive(s)")
	cmd.Aliases = []string{"dl"}
	cmd.Config = func(c *gcli.Command) {
		c.StrOpt(&opts.OS, "os", "", "", "Target OS, default current OS")
		c.StrOpt(&opts.Arch, "arch", "", "", "Target arch, default current arch")
		c.StrOpt(&opts.Output, "output", "o", "", "Directory to place downloaded archive(s)")
		c.AddArg("target", "SDK target(s), for example go@1.22.0 or node:20.11.1", true, true)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		targetArgs := append(c.Arg("target").Strings(), args...)
		if err := validateNoFlagArgs(targetArgs); err != nil {
			return err
		}
		opts.Targets = splitTargets(targetArgs)
		snapshot := *opts
		snapshot.Targets = append([]string(nil), opts.Targets...)
		return handler("sdk.download", &snapshot)
	}
	return cmd
}

func newSDKInstallCmd(opts *SDKInstallOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("install", "Install SDK target(s)")
	cmd.Aliases = []string{"i", "ins"}
	cmd.Config = func(c *gcli.Command) {
		c.BoolOpt(&opts.Force, "force", "f", false, "Remove existing SDK directory before installing")
		c.AddArg("target", "SDK target(s), for example go@1.22.0 or node:20.11.1", true, true)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		targetArgs := append(c.Arg("target").Strings(), args...)
		if err := validateNoFlagArgs(targetArgs); err != nil {
			return err
		}
		opts.Targets = splitTargets(targetArgs)
		snapshot := *opts
		snapshot.Targets = append([]string(nil), opts.Targets...)
		return handler("sdk.install", &snapshot)
	}
	return cmd
}

func newSDKListCmd(opts *SDKListOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("list", "List installed SDK versions")
	cmd.Aliases = []string{"ls"}
	cmd.Config = func(c *gcli.Command) {
		c.BoolOpt(&opts.JSON, "json", "j", false, "Output as JSON")
		c.AddArg("name", "SDK name filter", false)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		name := c.Arg("name").String()
		if err := validateNoFlagArgs(append([]string{name}, args...)); err != nil {
			return err
		}
		opts.Name = name
		snapshot := *opts
		return handler("sdk.list", &snapshot)
	}
	return cmd
}

func newSDKRemoveCmd(opts *SDKRemoveOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("remove", "Remove an installed SDK version")
	cmd.Aliases = []string{"rm"}
	cmd.Config = func(c *gcli.Command) {
		c.AddArg("target", "SDK target with explicit version", true)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		opts.Target = c.Arg("target").String()
		if err := validateNoFlagArgs(append([]string{opts.Target}, args...)); err != nil {
			return err
		}
		snapshot := *opts
		return handler("sdk.remove", &snapshot)
	}
	return cmd
}

func newSDKPathCmd(opts *SDKPathOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("path", "Print SDK path")
	cmd.Config = func(c *gcli.Command) {
		c.AddArg("target", "SDK target, for example go, go:1.20, or java:17", true)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		opts.Target = c.Arg("target").String()
		if err := validateNoFlagArgs(append([]string{opts.Target}, args...)); err != nil {
			return err
		}
		if len(args) > 0 {
			return fmt.Errorf("too many arguments: %v", args)
		}
		snapshot := *opts
		return handler("sdk.path", &snapshot)
	}
	return cmd
}

func newSDKSearchCmd(opts *SDKSearchOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("search", "Search SDK index cache")
	cmd.Config = func(c *gcli.Command) {
		c.BoolOpt(&opts.JSON, "json", "j", false, "Output as JSON")
		c.IntOpt(&opts.Number, "number", "n", 20, "Maximum search results, <= 0 means unlimited")
		c.StrOpt(&opts.Sort, "sort", "", "", "Sort by version: asc, desc")
		c.BoolOpt(&opts.Prerelease, "prerelease", "", false, "Include prerelease (non-stable) versions")
		c.AddArg("name", "SDK name", true)
		c.AddArg("keyword", "Search keyword(s), prefix with ^ to exclude", false, true)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		opts.Name = c.Arg("name").String()
		keywords := append(c.Arg("keyword").Strings(), args...)
		if err := validateNoFlagArgs(append([]string{opts.Name}, keywords...)); err != nil {
			return err
		}
		opts.Keywords = append([]string(nil), keywords...)
		snapshot := *opts
		snapshot.Keywords = append([]string(nil), opts.Keywords...)
		return handler("sdk.search", &snapshot)
	}
	return cmd
}

func newSDKIndexCmd(opts *SDKIndexOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("index", "Manage SDK index cache")
	cmd.Aliases = []string{"idx"}
	cmd.Subs = []*gcli.Command{
		newSDKIndexActionCmd("list", opts, handler, func(c *gcli.Command) {
			c.Aliases = []string{"ls"}
		}),
		newSDKIndexActionCmd("show", opts, handler, nil),
		newSDKIndexActionCmd("refresh", opts, handler, func(c *gcli.Command) {
			c.Aliases = []string{"build"}
		}),
		newSDKIndexActionCmd("clear", opts, handler, nil),
	}
	return cmd
}

func newSDKConfigCmd(opts *SDKConfigOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("config", "Manage SDK config templates")
	cmd.Aliases = []string{"cfg"}
	cmd.Subs = []*gcli.Command{
		newSDKConfigAddCmd(opts, handler),
	}
	return cmd
}

func newSDKConfigAddCmd(opts *SDKConfigOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("add", "Add built-in SDK config template")
	cmd.Config = func(c *gcli.Command) {
		c.BoolOpt(&opts.All, "all", "a", false, "Add all built-in SDK configs")
		c.BoolOpt(&opts.Force, "force", "f", false, "Overwrite existing SDK config")
		c.StrOpt(&opts.Mirror, "mirror", "m", "", "Use named built-in mirror source: mirror, aliyun, huawei, zulu")
		c.AddArg("name", "Built-in SDK name or alias", false)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		opts.Action = "add"
		opts.Name = c.Arg("name").String()
		if err := applySDKConfigAddTrailingFlags(opts, args); err != nil {
			return err
		}
		if (opts.Name == "") == !opts.All {
			return fmt.Errorf("sdk config add requires exactly one of <name> or --all")
		}
		snapshot := *opts
		return handler("sdk.config.add", &snapshot)
	}
	return cmd
}

func applySDKConfigAddTrailingFlags(opts *SDKConfigOptions, args []string) error {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--all", "-a":
			opts.All = true
		case "--force", "-f":
			opts.Force = true
		case "--mirror", "-m":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return fmt.Errorf("%s requires a value", arg)
			}
			i++
			opts.Mirror = args[i]
		default:
			if value, ok := strings.CutPrefix(arg, "--mirror="); ok {
				if value == "" {
					return fmt.Errorf("--mirror requires a value")
				}
				opts.Mirror = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "-m="); ok {
				if value == "" {
					return fmt.Errorf("-m requires a value")
				}
				opts.Mirror = value
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("option provided but not defined: %s", arg)
			}
			return fmt.Errorf("unexpected argument: %s", arg)
		}
	}
	return nil
}

func newSDKIndexActionCmd(action string, opts *SDKIndexOptions, handler CommandHandler, configure func(*gcli.Command)) *gcli.Command {
	cmd := gcli.NewCommand(action, "Run sdk index "+action)
	if configure != nil {
		configure(cmd)
	}
	cmd.Config = func(c *gcli.Command) {
		if action == "refresh" || action == "clear" {
			c.BoolOpt(&opts.All, "all", "a", false, "Apply to all SDK indexes")
		}
		if action == "list" {
			c.BoolOpt(&opts.JSON, "json", "j", false, "Output as JSON")
		}
		if action == "show" || action == "refresh" || action == "clear" {
			c.AddArg("name", "SDK name", action == "show")
		}
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		opts.Action = action
		if action == "show" || action == "refresh" || action == "clear" {
			opts.Name = c.Arg("name").String()
		}
		if err := validateNoFlagArgs(append([]string{opts.Name}, args...)); err != nil {
			return err
		}
		if (action == "refresh" || action == "clear") && ((opts.Name == "") == !opts.All) {
			return fmt.Errorf("sdk index %s requires exactly one of <name> or --all", action)
		}
		snapshot := *opts
		return handler("sdk.index."+action, &snapshot)
	}
	return cmd
}
