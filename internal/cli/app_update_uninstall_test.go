package cli

import (
	"bytes"
	"io"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestMain_RemoveBindsYesFlag(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"remove", "--yes", "gookit/gitw"})
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(calls))
	assert.Eq(t, "uninstall", calls[0].name)
	opts, ok := calls[0].options.(*UninstallOptions)
	assert.True(t, ok)
	assert.Eq(t, "gookit/gitw", opts.Target)
	assert.True(t, opts.Yes)
}

func TestMain_RemoveBindsMultipleTargets(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"rm", "--yes", "fzf", "rg"})
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(calls))
	opts, ok := calls[0].options.(*UninstallOptions)
	assert.True(t, ok)
	assert.Eq(t, []string{"fzf", "rg"}, opts.Targets)
	assert.True(t, opts.Yes)
}

func TestMain_UpdateBindsMultipleTargets(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"space separated", []string{"update", "fzf", "rg"}, []string{"fzf", "rg"}},
		{"comma separated", []string{"update", "fzf,rg,fd"}, []string{"fzf", "rg", "fd"}},
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
			assert.NoErr(t, err)
			assert.Eq(t, 1, len(calls))
			opts, ok := calls[0].options.(*UpdateOptions)
			assert.True(t, ok)
			assert.Eq(t, tt.want, opts.Targets)
		})
	}
}

func TestUpdateSelfFlagParses(t *testing.T) {
	var got *UpdateOptions
	app := newApp(func(name string, options any) error {
		if name != "update" {
			t.Fatalf("expected update command, got %q", name)
		}
		got = options.(*UpdateOptions)
		return nil
	}, io.Discard, io.Discard)

	err := app.RunWithArgs([]string{"update", "--self"})

	assert.NoErr(t, err)
	assert.True(t, got.Self)
	assert.Eq(t, 0, len(got.Targets))
}

func TestUpdateSelfSourceFlagParses(t *testing.T) {
	var got *UpdateOptions
	app := newApp(func(name string, options any) error {
		if name != "update" {
			t.Fatalf("expected update command, got %q", name)
		}
		got = options.(*UpdateOptions)
		return nil
	}, io.Discard, io.Discard)

	err := app.RunWithArgs([]string{"update", "--self", "--self-source", "https://example.com/tools/eget/"})

	assert.NoErr(t, err)
	assert.True(t, got.Self)
	assert.Eq(t, "https://example.com/tools/eget/", got.SelfSource)
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
	if len(updateFirst.Targets) != 1 || updateFirst.Targets[0] != "foo" {
		t.Fatalf("expected first update target foo, got %#v", updateFirst.Targets)
	}
	if len(updateSecond.Targets) != 0 {
		t.Fatalf("expected second update target to reset, got %#v", updateSecond.Targets)
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

func TestMain_UpdateInteractiveShortFlagParses(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"up", "-i"})
	if err != nil {
		t.Fatalf("expected update -i command to parse, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one handler call, got %d", len(calls))
	}

	opts, ok := calls[0].options.(*UpdateOptions)
	if !ok {
		t.Fatalf("expected UpdateOptions, got %T", calls[0].options)
	}
	if !opts.Interactive {
		t.Fatalf("expected interactive flag to be true")
	}
}
