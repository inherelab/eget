package cli

import (
	"fmt"

	"github.com/gookit/gcli/v3"
	"github.com/inherelab/eget/internal/app/web"
)

type WebOptions struct {
	Host           string
	Port           int
	Token          string
	NoAuth         bool
	ReadOnly       bool
	AllowMutations bool
	Open           bool
	NoTokenPrint   bool
	CacheRoot      string
	NoCacheIndex   bool
	AllowHosts     string
	JSONLog        bool
}

func newWebCmd(handler CommandHandler) (*gcli.Command, func()) {
	// The reset closure runs before every invocation and must restore the
	// declared defaults: gcli writes a flag default into the pointer when the
	// flag is registered, not on each parse.
	opts := &WebOptions{Port: web.DefaultPort}
	cmd := gcli.NewCommand("web", "Serve the eget web console over HTTP")
	cmd.Help = `<info>Examples</>:
  eget web
  eget web --open
  eget web --port 9000 --read-only
  eget web --host 0.0.0.0 --token "$EGET_WEB_TOKEN" --allow-mutations`
	cmd.Config = func(c *gcli.Command) {
		c.StrOpt(&opts.Host, "host", "", "", "Listen host (default 127.0.0.1, or [web].host); a non-loopback host requires a token")
		c.IntOpt(&opts.Port, "port", "p", web.DefaultPort, "Listen port, 0 means a random free port")
		c.StrOpt(&opts.Token, "token", "", "", "Bearer token for the console and the cache mirror; generated when empty")
		c.BoolOpt(&opts.NoAuth, "no-auth", "", false, "Disable token checks (loopback hosts only)")
		c.BoolOpt(&opts.ReadOnly, "read-only", "", false, "Serve read endpoints only")
		c.BoolOpt(&opts.AllowMutations, "allow-mutations", "", false, "Required to enable write endpoints on a non-loopback host")
		c.BoolOpt(&opts.Open, "open", "", false, "Open the console in the default browser")
		c.BoolOpt(&opts.NoTokenPrint, "no-token-print", "", false, "Do not print the token on startup")
		c.StrOpt(&opts.CacheRoot, "cache-root", "", "", "Cache mirror scope: all, pkg, api, sdk, sdk-index (default all, or [web].cache_root)")
		c.BoolOpt(&opts.NoCacheIndex, "no-cache-index", "", false, "Disable cache directory listing")
		c.StrOpt(&opts.AllowHosts, "allow-host", "", "", "Extra Host header names, comma separated")
		c.BoolOpt(&opts.JSONLog, "json-log", "", false, "Write one JSON request log line per request")
	}
	cmd.Func = func(_ *gcli.Command, args []string) error {
		if err := validateNoFlagArgs(args); err != nil {
			return err
		}
		if !isValidCacheRoot(opts.CacheRoot) {
			return fmt.Errorf("invalid cache root %q: must be one of all, pkg, api, sdk, sdk-index", opts.CacheRoot)
		}
		if opts.NoAuth && !web.IsLoopbackHost(opts.Host) {
			return fmt.Errorf("--no-auth is only allowed on a loopback host")
		}
		snapshot := *opts
		return handler("web", &snapshot)
	}
	return cmd, func() { *opts = WebOptions{Port: web.DefaultPort} }
}
