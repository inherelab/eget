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
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
)

func (s *cliService) handleWeb(opts *WebOptions) error {
	// [web] supplies defaults; command flags win.
	webCfg := s.webSection()
	resolved := webResolved{
		Host:           webFirstNonEmpty(opts.Host, webDerefString(webCfg.Host), "127.0.0.1"),
		Port:           opts.Port,
		ReadOnly:       opts.ReadOnly || webDerefBool(webCfg.ReadOnly),
		AllowMutations: opts.AllowMutations || webDerefBool(webCfg.AllowMutations),
		AutoOpen:       opts.Open || webDerefBool(webCfg.AutoOpen),
		CacheRoot:      webFirstNonEmpty(opts.CacheRoot, webDerefString(webCfg.CacheRoot), "all"),
		NoCacheIndex:   opts.NoCacheIndex || webDerefBool(webCfg.NoCacheIndex),
	}

	host := strings.TrimSpace(resolved.Host)
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
		Root:    resolved.CacheRoot,
		NoIndex: resolved.NoCacheIndex,
		Version: BuildInfo().Version,
	}
	// Loopback clients may manage packages by default; a non-loopback listener
	// requires the explicit --allow-mutations flag.
	allowMutations := resolved.AllowMutations || web.IsLoopbackHost(host)

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

		AssetCandidates: s.webAssetCandidates,
	}, web.Options{
		Host:           host,
		Port:           resolved.Port,
		Token:          token,
		ReadOnly:       resolved.ReadOnly,
		AllowMutations: allowMutations,
		Version:        BuildInfo().Version,
		CacheRoot:      resolved.CacheRoot,
		NoCacheIndex:   resolved.NoCacheIndex,
		AllowHosts:     splitCommaList(opts.AllowHosts),
		JSONLog:        opts.JSONLog,
		LogWriter:      s.stderrWriter(),
		TaskStore:      webTaskStorePath(),
		TaskRunners:    s.webTaskRunners(),
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	onReady := func(addr string) {
		s.printWebStartup(addr, cacheDir, token, generated, allowMutations, resolved, opts)
		if !resolved.AutoOpen {
			return
		}
		if err := openInBrowser(webConsoleURL(addr, token)); err != nil {
			ccolor.Warnf("open browser: %v\n", err)
		}
	}
	return server.Serve(ctx, onReady)
}

// webAssetCandidates lists the assets matching a target without downloading
// anything, which is how the install form offers an explicit choice.
func (s *cliService) webAssetCandidates(ctx context.Context, target string) ([]string, error) {
	if s.installService == nil {
		return nil, fmt.Errorf("the install service is unavailable")
	}
	runner := install.NewRunner(s.installService)
	return runner.ListAssetCandidates(target, install.Options{Context: ctx})
}

func (s *cliService) printWebStartup(addr, cacheDir, token string, generated, allowMutations bool, resolved webResolved, opts *WebOptions) {
	out := s.stderrWriter()
	root := resolved.CacheRoot
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
		ccolor.Fprintf(out, " - open:  <green>%s</>\n", webConsoleURL(addr, token))
	default:
		ccolor.Fprintf(out, " - token: from --token\n")
		ccolor.Fprintf(out, " - open:  %s\n", webConsoleURL(addr, token))
	}
	if resolved.ReadOnly {
		ccolor.Fprintf(out, " - mode: <ylw>read-only</>\n")
	} else if allowMutations {
		ccolor.Fprintf(out, " - mode: read-write\n")
	} else {
		ccolor.Fprintf(out, " - mode: <ylw>read-only</> (add --allow-mutations to enable writes on %s)\n", resolved.Host)
	}
	if !web.IsLoopbackHost(resolved.Host) {
		ccolor.Warnf(" - warning: listening on %s over plain HTTP with no TLS; put it behind a TLS reverse proxy\n", resolved.Host)
	}
}

// webResolved is the effective console configuration after merging [web] with
// the command flags.
type webResolved struct {
	Host           string
	Port           int
	ReadOnly       bool
	AllowMutations bool
	AutoOpen       bool
	CacheRoot      string
	NoCacheIndex   bool
}

func (s *cliService) webSection() cfgpkg.WebSection {
	cfg, err := s.cfgService.ConfigList()
	if err != nil || cfg == nil {
		return cfgpkg.WebSection{}
	}
	return cfg.Web
}

func webFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func webDerefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func webDerefBool(value *bool) bool {
	return value != nil && *value
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
