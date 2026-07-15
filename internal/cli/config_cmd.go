package cli

import (
	"fmt"

	"github.com/gookit/gcli/v3"
)

const supportedConfigPathTargets = "config_file, config_dir, env_file, bin_dir, cache_dir, sdk_dir, pkg_store_file, sdk_store_file"

type ConfigOptions struct {
	Action     string
	Key        string
	Value      string
	Target     string
	File       string
	Check      bool
	WithGlobal bool
	Force      bool
}

func newConfigCmd(handler CommandHandler) (*gcli.Command, func()) {
	opts := &ConfigOptions{}
	cmd := gcli.NewCommand("config", "Manage configuration")
	cmd.Aliases = []string{"cfg"}
	cmd.Help = `  init                Initialize the config file with default values
  list | ls           Print current config values and file status
  doctor              Print local paths and environment diagnostics
  path [--check] [target]
  export [--with-global] [FILE]
  import [--force] FILE

<info>Examples</>:
  eget config init
  eget config list
  eget config doctor
  eget cfg path cache_dir
  eget cfg path --check sdk_store_file
  eget config export portable.toml
  eget config import --force portable.toml
  eget config get global.target
  eget config set global.target ~/.local/bin`

	cmd.Subs = []*gcli.Command{
		newConfigActionCmd("init", nil, opts, handler),
		newConfigActionCmd("list", []string{"ls"}, opts, handler),
		newConfigActionCmd("doctor", nil, opts, handler),
		newConfigPathCmd(opts, handler),
		newConfigExportCmd(opts, handler),
		newConfigImportCmd(opts, handler),
		newConfigGetCmd(opts, handler),
		newConfigSetCmd(opts, handler),
	}
	return cmd, func() {
		*opts = ConfigOptions{}
	}
}

func newConfigExportCmd(opts *ConfigOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("export", "Export configuration as TOML")
	cmd.Config = func(c *gcli.Command) {
		c.BoolOpt(&opts.WithGlobal, "with-global", "", false, "Include the machine-specific global section")
		c.AddArg("file", "Output file, defaults to stdout", false)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		opts.Action = "export"
		opts.File = c.Arg("file").String()
		if err := validateNoFlagArgs(append([]string{opts.File}, args...)); err != nil {
			return err
		}
		if len(args) > 0 {
			return fmt.Errorf("too many arguments: %v", args)
		}
		snapshot := *opts
		return handler("config", &snapshot)
	}
	return cmd
}

func newConfigImportCmd(opts *ConfigOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("import", "Import configuration from TOML")
	cmd.Config = func(c *gcli.Command) {
		c.BoolOpt(&opts.Force, "force", "f", false, "Replace existing configuration without confirmation")
		c.AddArg("file", "TOML file to import", true)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		opts.Action = "import"
		opts.File = c.Arg("file").String()
		if err := validateNoFlagArgs(append([]string{opts.File}, args...)); err != nil {
			return err
		}
		if len(args) > 0 {
			return fmt.Errorf("too many arguments: %v", args)
		}
		snapshot := *opts
		return handler("config", &snapshot)
	}
	return cmd
}

func newConfigActionCmd(action string, aliases []string, opts *ConfigOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand(action, "Run config "+action)
	cmd.Aliases = aliases
	cmd.Func = func(_ *gcli.Command, args []string) error {
		if err := validateNoFlagArgs(args); err != nil {
			return err
		}
		opts.Action = action
		snapshot := *opts
		return handler("config", &snapshot)
	}
	return cmd
}

func newConfigPathCmd(opts *ConfigOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("path", "Print local config path")
	cmd.Config = func(c *gcli.Command) {
		c.BoolOpt(&opts.Check, "check", "", false, "Print path and existence status")
		c.AddArg("target", "Path target. Supported: "+supportedConfigPathTargets, false)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		opts.Action = "path"
		opts.Target = c.Arg("target").String()
		if opts.Target == "" {
			opts.Target = "config_file"
		}
		if err := validateNoFlagArgs(append([]string{opts.Target}, args...)); err != nil {
			return err
		}
		if len(args) > 0 {
			return fmt.Errorf("too many arguments: %v", args)
		}
		snapshot := *opts
		return handler("config", &snapshot)
	}
	return cmd
}

func newConfigGetCmd(opts *ConfigOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("get", "Print one config value")
	cmd.Config = func(c *gcli.Command) {
		c.AddArg("key", "Config key", true)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		opts.Action = "get"
		opts.Key = c.Arg("key").String()
		if err := validateNoFlagArgs(append([]string{opts.Key}, args...)); err != nil {
			return err
		}
		snapshot := *opts
		return handler("config", &snapshot)
	}
	return cmd
}

func newConfigSetCmd(opts *ConfigOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("set", "Update one config value")
	cmd.Config = func(c *gcli.Command) {
		c.AddArg("key", "Config key", true)
		c.AddArg("value", "Config value", true)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		opts.Action = "set"
		opts.Key = c.Arg("key").String()
		opts.Value = c.Arg("value").String()
		if err := validateNoFlagArgs(append([]string{opts.Key, opts.Value}, args...)); err != nil {
			return err
		}
		snapshot := *opts
		return handler("config", &snapshot)
	}
	return cmd
}
