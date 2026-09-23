package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/gookit/goutil/x/ccolor"
	app "github.com/inherelab/eget/internal/app"
	appcache "github.com/inherelab/eget/internal/app/cache"
	"github.com/inherelab/eget/internal/app/web"
)

func (s *cliService) handleWeb(opts *WebOptions) error {
	host := strings.TrimSpace(opts.Host)
	token := strings.TrimSpace(opts.Token)
	generated := false
	if token == "" && !opts.NoAuth {
		// A generated token is convenient for a local console, but a listener
		// reachable from other machines must be an explicit choice.
		if !web.IsLoopbackHost(host) {
			return fmt.Errorf("listening on %s requires an explicit --token (or use a loopback host)", host)
		}
		var err error
		if token, err = newWebToken(); err != nil {
			return err
		}
		generated = true
	}

	cacheDir, err := s.cacheService.ResolveCacheDir()
	if err != nil {
		return err
	}
	machineOpts := appcache.MachineOptions{
		Root:    opts.CacheRoot,
		NoIndex: opts.NoCacheIndex,
		Version: BuildInfo().Version,
	}
	// Loopback clients may manage packages by default; a non-loopback listener
	// requires the explicit --allow-mutations flag.
	allowMutations := opts.AllowMutations || web.IsLoopbackHost(host)

	// The console always includes packages owned by external managers. The CLI
	// default (ext_package_mode = off) stays untouched: ListService is a value
	// type, so this copy cannot leak into other commands.
	webList := s.listService
	webList.Managers = app.ManagersSelection{Mode: app.ManagersModeWith}

	server, err := web.NewServer(web.Deps{
		List:     webList,
		Show:     s.showService,
		Query:    s.queryService,
		Search:   s.searchService,
		Config:   s.cfgService,
		Cache:    s.cacheService,
		Ext:      s.extPkg,
		Manifest: appcache.ManifestHandler(s.cacheService, cacheDir, machineOpts),
		Download: appcache.DownloadHandler(s.cacheService, cacheDir, machineOpts),
		File:     appcache.FileHandler(s.cacheService, cacheDir, machineOpts),
	}, web.Options{
		Host:           host,
		Port:           opts.Port,
		Token:          token,
		ReadOnly:       opts.ReadOnly,
		AllowMutations: allowMutations,
		Version:        BuildInfo().Version,
		CacheRoot:      opts.CacheRoot,
		NoCacheIndex:   opts.NoCacheIndex,
		AllowHosts:     splitCommaList(opts.AllowHosts),
		JSONLog:        opts.JSONLog,
		LogWriter:      s.stderrWriter(),
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	onReady := func(addr string) {
		s.printWebStartup(addr, cacheDir, token, generated, allowMutations, opts)
		if !opts.Open {
			return
		}
		if err := openInBrowser(webConsoleURL(addr, token)); err != nil {
			ccolor.Warnf("open browser: %v\n", err)
		}
	}
	return server.Serve(ctx, onReady)
}

func (s *cliService) printWebStartup(addr, cacheDir, token string, generated, allowMutations bool, opts *WebOptions) {
	out := s.stderrWriter()
	root := strings.TrimSpace(opts.CacheRoot)
	if root == "" {
		root = "all"
	}
	ccolor.Fprintf(out, "Serving eget web console on <green>http://%s</>\n", addr)
	if info, err := s.cfgService.ConfigInfo(); err == nil {
		exists := "missing"
		if info.Exists {
			exists = "found"
		}
		ccolor.Fprintf(out, " - config: %s (%s)\n", info.Path, exists)
	}
	ccolor.Fprintf(out, " - cache dir: %s (mirror scope: %s)\n", cacheDir, root)
	ccolor.Fprintf(out, " - cache mirror: /manifest.json, /download/*, /files/*\n")
	switch {
	case opts.NoAuth:
		ccolor.Fprintf(out, " - auth: <ylw>disabled</> (loopback only)\n")
	case generated && opts.NoTokenPrint:
		ccolor.Fprintf(out, " - token: <ylw>hidden</> (pass --token or open the console from this terminal)\n")
	case generated:
		ccolor.Fprintf(out, " - token: <green>%s</>\n", token)
	default:
		ccolor.Fprintf(out, " - token: from --token\n")
	}
	if opts.ReadOnly {
		ccolor.Fprintf(out, " - mode: <ylw>read-only</>\n")
	} else if allowMutations {
		ccolor.Fprintf(out, " - mode: read-write\n")
	} else {
		ccolor.Fprintf(out, " - mode: <ylw>read-only</> (add --allow-mutations to enable writes on %s)\n", opts.Host)
	}
	if !web.IsLoopbackHost(opts.Host) {
		ccolor.Warnf(" - warning: listening on %s over plain HTTP with no TLS; put it behind a TLS reverse proxy\n", opts.Host)
	}
}

// webConsoleURL builds the browser URL. With a random port the bound address is
// only known after Listen, and a wildcard host is not browsable.
func webConsoleURL(addr, token string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr + "/"
	}
	switch host {
	case "", "0.0.0.0", "::":
		host = "127.0.0.1"
	}
	console := "http://" + net.JoinHostPort(host, port) + "/"
	if token != "" {
		console += "?token=" + url.QueryEscape(token)
	}
	return console
}

func newWebToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func openInBrowser(target string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
	case "darwin":
		return exec.Command("open", target).Start()
	default:
		return exec.Command("xdg-open", target).Start()
	}
}

func splitCommaList(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
