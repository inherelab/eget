package cli

import (
	"bytes"
	"errors"
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
	if err == nil {
		t.Fatalf("expected error for missing subcommand")
	}
	if !errors.Is(err, ErrCommandRequired) {
		t.Fatalf("expected ErrCommandRequired, got %v", err)
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

func TestMain_InstallRejectsFlagsAfterTarget(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Main([]string{"install", "inhere/markview", "--tag", "nightly"}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected parse error for trailing flags after target")
	}
	if errors.Is(err, ErrCommandRequired) {
		t.Fatalf("expected parse error, got %v", err)
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
