package cli

import (
	"bytes"
	"strings"
	"testing"
)

type commandCall struct {
	name    string
	options any
}

func TestMain_NoSubcommandReturnsErrorAndHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Main([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error for missing subcommand, got %v", err)
	}
	if stdout.Len() == 0 {
		t.Fatalf("expected help output on stdout")
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("expected help output to contain usage, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected stderr to be empty, got %q", stderr.String())
	}
}

func TestMain_InstallStandardOrderRoutesAndBindsOptions(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"install", "--tag", "nightly", "inhere/markview"})
	if err != nil {
		t.Fatalf("expected install command to parse, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one handler call, got %d", len(calls))
	}
	if calls[0].name != "install" {
		t.Fatalf("expected command install, got %q", calls[0].name)
	}

	opts, ok := calls[0].options.(*InstallOptions)
	if !ok {
		t.Fatalf("expected InstallOptions, got %T", calls[0].options)
	}
	if opts.Tag != "nightly" {
		t.Fatalf("expected tag nightly, got %q", opts.Tag)
	}
	if opts.Target != "inhere/markview" {
		t.Fatalf("expected target inhere/markview, got %q", opts.Target)
	}
}

func TestMain_ExtractAllFlagBindsInstallDownloadAndAdd(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"install long", []string{"install", "--extract-all", "inhere/markview"}, "install"},
		{"install short", []string{"install", "--ea", "inhere/markview"}, "install"},
		{"download long", []string{"download", "--extract-all", "inhere/markview"}, "download"},
		{"download short", []string{"download", "--ea", "inhere/markview"}, "download"},
		{"add long", []string{"add", "--extract-all", "inhere/markview"}, "add"},
		{"add short", []string{"add", "--ea", "inhere/markview"}, "add"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := make([]commandCall, 0, 1)
			handler := func(name string, options any) error {
				calls = append(calls, commandCall{name: name, options: options})
				return nil
			}

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := newApp(handler, &stdout, &stderr).RunWithArgs(tt.args)
			if err != nil {
				t.Fatalf("expected %s command to parse, got %v", tt.name, err)
			}
			if len(calls) != 1 || calls[0].name != tt.want {
				t.Fatalf("unexpected routed call: %#v", calls)
			}
			switch opts := calls[0].options.(type) {
			case *InstallOptions:
				if !opts.All {
					t.Fatalf("expected install extract-all flag to be true")
				}
			case *DownloadOptions:
				if !opts.All {
					t.Fatalf("expected download extract-all flag to be true")
				}
			case *AddOptions:
				if !opts.All {
					t.Fatalf("expected add extract-all flag to be true")
				}
			default:
				t.Fatalf("unexpected options type %T", calls[0].options)
			}
		})
	}
}

func TestMain_GUIFlagBindsInstallAndAdd(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"install gui", []string{"install", "--gui", "inhere/markview"}, "install"},
		{"add gui", []string{"add", "--gui", "inhere/markview"}, "add"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := make([]commandCall, 0, 1)
			handler := func(name string, options any) error {
				calls = append(calls, commandCall{name: name, options: options})
				return nil
			}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := newApp(handler, &stdout, &stderr).RunWithArgs(tt.args)
			if err != nil {
				t.Fatalf("expected %s command to parse, got %v", tt.name, err)
			}
			if len(calls) != 1 || calls[0].name != tt.want {
				t.Fatalf("unexpected routed call: %#v", calls)
			}
			switch opts := calls[0].options.(type) {
			case *InstallOptions:
				if !opts.GUI {
					t.Fatalf("expected install gui flag to be true")
				}
			case *AddOptions:
				if !opts.GUI {
					t.Fatalf("expected add gui flag to be true")
				}
			default:
				t.Fatalf("unexpected options type %T", calls[0].options)
			}
		})
	}
}

func TestMain_DownloadRejectsGUIFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(func(string, any) error { return nil }, &stdout, &stderr).RunWithArgs([]string{"download", "--gui", "inhere/markview"})
	if err == nil {
		t.Fatal("expected download --gui to be rejected")
	}
	if !strings.Contains(err.Error(), "gui") {
		t.Fatalf("expected error to mention gui, got %v", err)
	}
}

func TestMain_InstallDownloadAndAddRejectRemovedAllFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"install", []string{"install", "--all", "inhere/markview"}},
		{"download", []string{"download", "--all", "inhere/markview"}},
		{"add", []string{"add", "--all", "inhere/markview"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := newApp(func(string, any) error { return nil }, &stdout, &stderr).RunWithArgs(tt.args)
			if err == nil {
				t.Fatalf("expected %s --all to be rejected", tt.name)
			}
			if !strings.Contains(err.Error(), "all") {
				t.Fatalf("expected error to mention all, got %v", err)
			}
		})
	}
}

func TestMain_InstallRejectsRemovedCacheDirFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Main([]string{"install", "--cache-dir", "~/.cache/eget", "inhere/markview"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected parse error for removed --cache-dir flag")
	}
	if !strings.Contains(err.Error(), "cache-dir") {
		t.Fatalf("expected error to mention cache-dir, got %v", err)
	}
}

func TestMain_InstallRejectsFlagsAfterTarget(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Main([]string{"install", "inhere/markview", "--tag", "nightly"}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected parse error for trailing flags after target")
	}
	if !strings.Contains(err.Error(), "flags must appear before arguments") {
		t.Fatalf("expected trailing-flag error, got %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected stderr to be empty, got %q", stderr.String())
	}
}

func TestMain_ConfigActionRoutesToConfigCommand(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"config", "list"})
	if err != nil {
		t.Fatalf("expected config command to parse, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one handler call, got %d", len(calls))
	}
	if calls[0].name != "config" {
		t.Fatalf("expected command config, got %q", calls[0].name)
	}

	opts, ok := calls[0].options.(*ConfigOptions)
	if !ok {
		t.Fatalf("expected ConfigOptions, got %T", calls[0].options)
	}
	if opts.Action != "list" {
		t.Fatalf("expected action list, got %q", opts.Action)
	}
}

func TestMain_GlobalVerboseFlagParsesBeforeCommand(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(handler, &stdout, &stderr)
	err := app.RunWithArgs([]string{"-v", "install", "inhere/markview"})
	if err != nil {
		t.Fatalf("expected verbose install command to parse, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one handler call, got %d", len(calls))
	}
	if !app.Verbose() {
		t.Fatalf("expected app verbose flag to be true")
	}
}

func TestMain_ListRoutesToListCommand(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"list"})
	if err != nil {
		t.Fatalf("expected list command to parse, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one handler call, got %d", len(calls))
	}
	if calls[0].name != "list" {
		t.Fatalf("expected command list, got %q", calls[0].name)
	}

	if _, ok := calls[0].options.(*ListOptions); !ok {
		t.Fatalf("expected ListOptions, got %T", calls[0].options)
	}
}

func TestMain_ListOutdatedBindsOption(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"list", "--outdated"})
	if err != nil {
		t.Fatalf("expected list --outdated command to parse, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one handler call, got %d", len(calls))
	}

	opts, ok := calls[0].options.(*ListOptions)
	if !ok {
		t.Fatalf("expected ListOptions, got %T", calls[0].options)
	}
	if !opts.Outdated {
		t.Fatalf("expected outdated flag to be true")
	}
}

func TestMain_ListAllBindsOption(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"list", "--all"})
	if err != nil {
		t.Fatalf("expected list --all command to parse, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one handler call, got %d", len(calls))
	}

	opts, ok := calls[0].options.(*ListOptions)
	if !ok {
		t.Fatalf("expected ListOptions, got %T", calls[0].options)
	}
	if !opts.All {
		t.Fatalf("expected all flag to be true")
	}
}

func TestMain_ListAllShortBindsOption(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"list", "-a"})
	if err != nil {
		t.Fatalf("expected list -a command to parse, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one handler call, got %d", len(calls))
	}

	opts, ok := calls[0].options.(*ListOptions)
	if !ok {
		t.Fatalf("expected ListOptions, got %T", calls[0].options)
	}
	if !opts.All {
		t.Fatalf("expected all flag to be true")
	}
}

func TestMain_ListGUIBindsOption(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"list", "--gui"})
	if err != nil {
		t.Fatalf("expected list --gui command to parse, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one handler call, got %d", len(calls))
	}

	opts, ok := calls[0].options.(*ListOptions)
	if !ok {
		t.Fatalf("expected ListOptions, got %T", calls[0].options)
	}
	if !opts.GUI {
		t.Fatalf("expected gui flag to be true")
	}
}

func TestMain_ListInfoBindsOption(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"list", "--info", "chlog"})
	if err != nil {
		t.Fatalf("expected list --info command to parse, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one handler call, got %d", len(calls))
	}

	opts, ok := calls[0].options.(*ListOptions)
	if !ok {
		t.Fatalf("expected ListOptions, got %T", calls[0].options)
	}
	if opts.Info != "chlog" {
		t.Fatalf("expected info option chlog, got %q", opts.Info)
	}
}

func TestMain_QueryRoutesAndBindsOptions(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"query", "--action", "releases", "--limit", "5", "--json", "owner/repo"})
	if err != nil {
		t.Fatalf("expected query command to parse, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one handler call, got %d", len(calls))
	}
	if calls[0].name != "query" {
		t.Fatalf("expected command query, got %q", calls[0].name)
	}

	opts, ok := calls[0].options.(*QueryOptions)
	if !ok {
		t.Fatalf("expected QueryOptions, got %T", calls[0].options)
	}
	if opts.Action != "releases" || opts.Limit != 5 || !opts.JSON || opts.Target != "owner/repo" {
		t.Fatalf("unexpected query options: %#v", opts)
	}
}

func TestMain_QueryAliasRoutes(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"q", "owner/repo"})
	if err != nil {
		t.Fatalf("expected query alias to parse, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one handler call, got %d", len(calls))
	}
	if calls[0].name != "query" {
		t.Fatalf("expected command query, got %q", calls[0].name)
	}
}

func TestMain_UninstallRoutesToUninstallCommandAndAliases(t *testing.T) {
	for _, name := range []string{"uninstall", "uni", "rm"} {
		t.Run(name, func(t *testing.T) {
			calls := make([]commandCall, 0, 1)
			handler := func(cmdName string, options any) error {
				calls = append(calls, commandCall{name: cmdName, options: options})
				return nil
			}

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{name, "fzf"})
			if err != nil {
				t.Fatalf("expected %s command to parse, got %v", name, err)
			}
			if len(calls) != 1 {
				t.Fatalf("expected one handler call, got %d", len(calls))
			}
			if calls[0].name != "uninstall" {
				t.Fatalf("expected command uninstall, got %q", calls[0].name)
			}

			opts, ok := calls[0].options.(*UninstallOptions)
			if !ok {
				t.Fatalf("expected UninstallOptions, got %T", calls[0].options)
			}
			if opts.Target != "fzf" {
				t.Fatalf("expected uninstall target fzf, got %q", opts.Target)
			}
		})
	}
}

func TestApp_RunWithArgsDoesNotLeakCommandStateAcrossRuns(t *testing.T) {
	calls := make([]commandCall, 0, 4)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(handler, &stdout, &stderr)

	if err := app.RunWithArgs([]string{"update", "foo"}); err != nil {
		t.Fatalf("expected first update run to succeed, got %v", err)
	}
	if err := app.RunWithArgs([]string{"update"}); err != nil {
		t.Fatalf("expected second update run to succeed, got %v", err)
	}
	if err := app.RunWithArgs([]string{"install", "--tag", "nightly", "inhere/markview"}); err != nil {
		t.Fatalf("expected first install run to succeed, got %v", err)
	}
	if err := app.RunWithArgs([]string{"install", "inhere/markview"}); err != nil {
		t.Fatalf("expected second install run to succeed, got %v", err)
	}

	if len(calls) != 4 {
		t.Fatalf("expected four handler calls, got %d", len(calls))
	}

	updateFirst, ok := calls[0].options.(*UpdateOptions)
	if !ok {
		t.Fatalf("expected first update options, got %T", calls[0].options)
	}
	updateSecond, ok := calls[1].options.(*UpdateOptions)
	if !ok {
		t.Fatalf("expected second update options, got %T", calls[1].options)
	}
	if updateFirst.Target != "foo" {
		t.Fatalf("expected first update target foo, got %q", updateFirst.Target)
	}
	if updateSecond.Target != "" {
		t.Fatalf("expected second update target to reset, got %q", updateSecond.Target)
	}

	installFirst, ok := calls[2].options.(*InstallOptions)
	if !ok {
		t.Fatalf("expected first install options, got %T", calls[2].options)
	}
	installSecond, ok := calls[3].options.(*InstallOptions)
	if !ok {
		t.Fatalf("expected second install options, got %T", calls[3].options)
	}
	if installFirst.Tag != "nightly" {
		t.Fatalf("expected first install tag nightly, got %q", installFirst.Tag)
	}
	if installSecond.Tag != "" {
		t.Fatalf("expected second install tag to reset, got %q", installSecond.Tag)
	}
}

func TestMain_UpdateCheckBindsOption(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"up", "--check"})
	if err != nil {
		t.Fatalf("expected update --check command to parse, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one handler call, got %d", len(calls))
	}
	if calls[0].name != "update" {
		t.Fatalf("expected command update, got %q", calls[0].name)
	}

	opts, ok := calls[0].options.(*UpdateOptions)
	if !ok {
		t.Fatalf("expected UpdateOptions, got %T", calls[0].options)
	}
	if !opts.Check {
		t.Fatalf("expected check flag to be true")
	}
}

func TestMain_SearchRoutesAndBindsOptions(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{
		"search", "--limit", "10", "--sort", "stars", "--order", "desc", "--json",
		"keyword", "user:junegunn", "language:go",
	})
	if err != nil {
		t.Fatalf("expected search command to parse, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one handler call, got %d", len(calls))
	}
	if calls[0].name != "search" {
		t.Fatalf("expected command search, got %q", calls[0].name)
	}

	opts, ok := calls[0].options.(*SearchOptions)
	if !ok {
		t.Fatalf("expected SearchOptions, got %T", calls[0].options)
	}
	if opts.Limit != 10 || opts.Sort != "stars" || opts.Order != "desc" || !opts.JSON {
		t.Fatalf("unexpected search flags: %#v", opts)
	}
	if opts.Keyword != "keyword" {
		t.Fatalf("expected keyword, got %q", opts.Keyword)
	}
	if len(opts.Extras) != 2 || opts.Extras[0] != "user:junegunn" || opts.Extras[1] != "language:go" {
		t.Fatalf("unexpected search extras: %#v", opts.Extras)
	}
}

func TestApp_RunWithArgsDoesNotLeakSearchStateAcrossRuns(t *testing.T) {
	calls := make([]commandCall, 0, 2)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(handler, &stdout, &stderr)

	if err := app.RunWithArgs([]string{"search", "--limit", "10", "--json", "keyword", "language:go"}); err != nil {
		t.Fatalf("expected first search run to succeed, got %v", err)
	}
	if err := app.RunWithArgs([]string{"search", "second"}); err != nil {
		t.Fatalf("expected second search run to succeed, got %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("expected two handler calls, got %d", len(calls))
	}

	first, ok := calls[0].options.(*SearchOptions)
	if !ok {
		t.Fatalf("expected first search options, got %T", calls[0].options)
	}
	second, ok := calls[1].options.(*SearchOptions)
	if !ok {
		t.Fatalf("expected second search options, got %T", calls[1].options)
	}

	if first.Limit != 10 || !first.JSON || first.Keyword != "keyword" || len(first.Extras) != 1 {
		t.Fatalf("unexpected first search options: %#v", first)
	}
	if second.Limit != 10 || second.JSON || second.Keyword != "second" || len(second.Extras) != 0 {
		t.Fatalf("expected second search options reset, got %#v", second)
	}
}
