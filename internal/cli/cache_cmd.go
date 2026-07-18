package cli

import (
	"fmt"

	"github.com/gookit/gcli/v3"
)

type CacheCleanOptions struct {
	Older      string
	All        bool
	DryRun     bool
	Yes        bool
	JSON       bool
	Pkg        bool
	API        bool
	SDK        bool
	SDKIndex   bool
	Partial    bool
	KeepLatest bool
}

type CacheListOptions struct {
	Root string
	JSON bool
}

type CacheStatusOptions struct {
	JSON bool
}

type CacheServeOptions struct {
	Host    string
	Port    int
	Root    string
	NoIndex bool
	Token   string
	JSONLog bool
}

func newCacheCmd(handler CommandHandler) (*gcli.Command, func()) {
	cleanOpts := &CacheCleanOptions{}
	serveOpts := &CacheServeOptions{Host: "0.0.0.0", Port: 8686, Root: "all"}
	cmd := gcli.NewCommand("cache", "Manage local eget cache")
	cmd.Aliases = []string{"ca"}
	cmd.Help = `<info>Examples</>:
  eget cache clean
  eget cache clean --dry-run --older 7d
  eget cache list --root sdk --json
  eget cache status
  eget cache clean --api --all
  eget cache serve
  eget cache serve --host 127.0.0.1 --port 0 --root sdk`
	cmd.Subs = []*gcli.Command{
		newCacheListCmd(&CacheListOptions{Root: "all"}, handler),
		newCacheStatusCmd(&CacheStatusOptions{}, handler),
		newCacheCleanCmd(cleanOpts, handler),
		newCacheServeCmd(serveOpts, handler),
	}
	return cmd, func() {
		*cleanOpts = CacheCleanOptions{}
		*serveOpts = CacheServeOptions{Host: "0.0.0.0", Port: 8686, Root: "all"}
	}
}

func newCacheListCmd(opts *CacheListOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("list", "List local cache files")
	cmd.Aliases = []string{"ls"}
	cmd.Config = func(c *gcli.Command) {
		c.StrOpt(&opts.Root, "root", "", "all", "Filter root: all, pkg, api, sdk, sdk-index, partial")
		c.BoolOpt(&opts.JSON, "json", "j", false, "Output as JSON")
	}
	cmd.Func = func(_ *gcli.Command, args []string) error {
		if err := validateNoFlagArgs(args); err != nil {
			return err
		}
		if !isValidCacheListRoot(opts.Root) {
			return fmt.Errorf("invalid cache root %q: must be one of all, pkg, api, sdk, sdk-index, partial", opts.Root)
		}
		snapshot := *opts
		return handler("cache.list", &snapshot)
	}
	return cmd
}

func newCacheStatusCmd(opts *CacheStatusOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("status", "Show local cache status")
	cmd.Aliases = []string{"st"}
	cmd.Config = func(c *gcli.Command) {
		c.BoolOpt(&opts.JSON, "json", "j", false, "Output as JSON")
	}
	cmd.Func = func(_ *gcli.Command, args []string) error {
		if err := validateNoFlagArgs(args); err != nil {
			return err
		}
		snapshot := *opts
		return handler("cache.status", &snapshot)
	}
	return cmd
}

func newCacheCleanCmd(opts *CacheCleanOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("clean", "Clean local cache files")
	cmd.Config = func(c *gcli.Command) {
		c.StrOpt(&opts.Older, "older", "", "", "Remove files older than duration, default 3d")
		c.BoolOpt(&opts.All, "all", "a", false, "Ignore older duration and remove all selected cache files")
		c.BoolOpt(&opts.KeepLatest, "keep-latest", "", false, "Keep the latest package cache versions")
		c.BoolOpt(&opts.DryRun, "dry-run", "", false, "Print matched files without removing them")
		c.BoolOpt(&opts.Yes, "yes", "y", false, "Skip large deletion confirmation")
		c.BoolOpt(&opts.JSON, "json", "j", false, "Output as JSON")
		c.BoolOpt(&opts.Pkg, "pkg", "", false, "Select package/download cache")
		c.BoolOpt(&opts.API, "api", "", false, "Select API cache")
		c.BoolOpt(&opts.SDK, "sdk", "", false, "Select SDK download cache")
		c.BoolOpt(&opts.SDKIndex, "sdk-index", "", false, "Select SDK index cache")
		c.BoolOpt(&opts.Partial, "partial", "", false, "Select unfinished download state")
	}
	cmd.Func = func(_ *gcli.Command, args []string) error {
		if err := validateNoFlagArgs(args); err != nil {
			return err
		}
		snapshot := *opts
		return handler("cache.clean", &snapshot)
	}
	return cmd
}

func newCacheServeCmd(opts *CacheServeOptions, handler CommandHandler) *gcli.Command {
	cmd := gcli.NewCommand("serve", "Serve local cache files over read-only HTTP")
	cmd.Config = func(c *gcli.Command) {
		c.StrOpt(&opts.Host, "host", "", "0.0.0.0", "Listen host")
		c.IntOpt(&opts.Port, "port", "p", 8686, "Listen port, 0 means random free port")
		c.StrOpt(&opts.Root, "root", "", "all", "Share scope: all, pkg, api, sdk, sdk-index")
		c.StrOpt(&opts.Token, "token", "", "", "Bearer token required for cache downloads and manifest")
		c.BoolOpt(&opts.NoIndex, "no-index", "", false, "Disable directory listing")
		c.BoolOpt(&opts.JSONLog, "json-log", "", false, "Write one JSON request log line per cache server request")
	}
	cmd.Func = func(_ *gcli.Command, args []string) error {
		if err := validateNoFlagArgs(args); err != nil {
			return err
		}
		if !isValidCacheRoot(opts.Root) {
			return fmt.Errorf("invalid cache root %q: must be one of all, pkg, api, sdk, sdk-index", opts.Root)
		}
		snapshot := *opts
		return handler("cache.serve", &snapshot)
	}
	return cmd
}

func isValidCacheListRoot(root string) bool {
	if root == "partial" {
		return true
	}
	return isValidCacheRoot(root)
}

func isValidCacheRoot(root string) bool {
	switch root {
	case "", "all", "pkg", "api", "sdk", "sdk-index":
		return true
	default:
		return false
	}
}
