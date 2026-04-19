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
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"install", "--tag", "nightly", "--cache-dir", "~/.cache/eget", "inhere/markview"})
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
	if opts.CacheDir != "~/.cache/eget" {
		t.Fatalf("expected cache dir ~/.cache/eget, got %q", opts.CacheDir)
	}
	if opts.Target != "inhere/markview" {
		t.Fatalf("expected target inhere/markview, got %q", opts.Target)
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

func TestMain_ConfigInfoRoutesToConfigCommand(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"config", "--info"})
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
	if !opts.Info {
		t.Fatalf("expected info flag to be true")
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
