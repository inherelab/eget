package cli

import (
	"errors"
	"testing"
)

func TestRun_NoSubcommandReturnsError(t *testing.T) {
	result := Run([]string{})

	if result.Err == nil {
		t.Fatalf("expected error for missing subcommand")
	}
	if !errors.Is(result.Err, ErrCommandRequired) {
		t.Fatalf("expected ErrCommandRequired, got %v", result.Err)
	}
}

func TestRun_InstallStandardOrder(t *testing.T) {
	result := Run([]string{"install", "--tag", "nightly", "inhere/markview"})

	if result.Err == nil {
		t.Fatalf("expected handler error for install skeleton")
	}
	if !errors.Is(result.Err, ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got %v", result.Err)
	}
	if result.Command != "install" {
		t.Fatalf("expected command install, got %q", result.Command)
	}

	opts, ok := result.Options.(*InstallOptions)
	if !ok {
		t.Fatalf("expected InstallOptions, got %T", result.Options)
	}
	if opts.Tag != "nightly" {
		t.Fatalf("expected tag nightly, got %q", opts.Tag)
	}
	if opts.Target != "inhere/markview" {
		t.Fatalf("expected target inhere/markview, got %q", opts.Target)
	}
}

func TestRun_InstallRejectsFlagsAfterTarget(t *testing.T) {
	result := Run([]string{"install", "inhere/markview", "--tag", "nightly"})

	if result.Err == nil {
		t.Fatalf("expected parse error for trailing flags after target")
	}
	if errors.Is(result.Err, ErrNotImplemented) {
		t.Fatalf("expected parse error, got handler error: %v", result.Err)
	}
}

func TestRun_ConfigInfoRoutesToConfigCommand(t *testing.T) {
	result := Run([]string{"config", "--info"})

	if result.Err == nil {
		t.Fatalf("expected handler error for config skeleton")
	}
	if !errors.Is(result.Err, ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got %v", result.Err)
	}
	if result.Command != "config" {
		t.Fatalf("expected command config, got %q", result.Command)
	}

	opts, ok := result.Options.(*ConfigOptions)
	if !ok {
		t.Fatalf("expected ConfigOptions, got %T", result.Options)
	}
	if !opts.Info {
		t.Fatalf("expected info flag to be true")
	}
}
