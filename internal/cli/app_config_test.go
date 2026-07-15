package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestMain_ConfigSubcommandsRouteToConfigCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		want      ConfigOptions
		wantCalls int
	}{
		{
			name:      "init",
			args:      []string{"config", "init"},
			want:      ConfigOptions{Action: "init"},
			wantCalls: 1,
		},
		{
			name:      "list",
			args:      []string{"config", "list"},
			want:      ConfigOptions{Action: "list"},
			wantCalls: 1,
		},
		{
			name:      "list alias",
			args:      []string{"config", "ls"},
			want:      ConfigOptions{Action: "list"},
			wantCalls: 1,
		},
		{
			name:      "doctor",
			args:      []string{"config", "doctor"},
			want:      ConfigOptions{Action: "doctor"},
			wantCalls: 1,
		},
		{
			name:      "path default",
			args:      []string{"cfg", "path"},
			want:      ConfigOptions{Action: "path", Target: "config_file"},
			wantCalls: 1,
		},
		{
			name:      "path check",
			args:      []string{"cfg", "path", "--check", "cache_dir"},
			want:      ConfigOptions{Action: "path", Target: "cache_dir", Check: true},
			wantCalls: 1,
		},
		{
			name:      "get",
			args:      []string{"config", "get", "global.target"},
			want:      ConfigOptions{Action: "get", Key: "global.target"},
			wantCalls: 1,
		},
		{
			name:      "set",
			args:      []string{"config", "set", "global.target", "~/.local/bin"},
			want:      ConfigOptions{Action: "set", Key: "global.target", Value: "~/.local/bin"},
			wantCalls: 1,
		},
		{
			name:      "export stdout",
			args:      []string{"config", "export", "--with-global"},
			want:      ConfigOptions{Action: "export", WithGlobal: true},
			wantCalls: 1,
		},
		{
			name:      "export file",
			args:      []string{"config", "export", "portable.toml"},
			want:      ConfigOptions{Action: "export", File: "portable.toml"},
			wantCalls: 1,
		},
		{
			name:      "import force",
			args:      []string{"config", "import", "portable.toml", "--force"},
			want:      ConfigOptions{Action: "import", File: "portable.toml", Force: true},
			wantCalls: 1,
		},
		{
			name:      "top alias",
			args:      []string{"cfg", "list"},
			want:      ConfigOptions{Action: "list"},
			wantCalls: 1,
		},
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
			assert.Eq(t, tt.wantCalls, len(calls))
			if len(calls) == 0 {
				return
			}
			assert.Eq(t, "config", calls[0].name)

			opts, ok := calls[0].options.(*ConfigOptions)
			assert.True(t, ok)
			assert.Eq(t, tt.want.Action, opts.Action)
			assert.Eq(t, tt.want.Key, opts.Key)
			assert.Eq(t, tt.want.Value, opts.Value)
			assert.Eq(t, tt.want.Target, opts.Target)
			assert.Eq(t, tt.want.Check, opts.Check)
			assert.Eq(t, tt.want.File, opts.File)
			assert.Eq(t, tt.want.WithGlobal, opts.WithGlobal)
			assert.Eq(t, tt.want.Force, opts.Force)
		})
	}
}

func TestMain_ConfigPathRejectsExtraArgs(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(func(string, any) error {
		t.Fatal("handler should not run")
		return nil
	}, &stdout, &stderr).RunWithArgs([]string{"cfg", "path", "cache_dir", "extra"})

	assert.Err(t, err)
	assert.Contains(t, err.Error(), "too many")
}

func TestMain_ConfigWithoutSubcommandShowsHelp(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"config"})
	assert.NoErr(t, err)
	assert.Eq(t, 0, len(calls))

	help := stdout.String()
	if !strings.Contains(help, "Available Commands") && !strings.Contains(help, "Usage") {
		t.Fatalf("expected config help output, got %q", help)
	}
	if !strings.Contains(help, "init") || !strings.Contains(help, "get") || !strings.Contains(help, "set") {
		t.Fatalf("expected config subcommands in help output, got %q", help)
	}
}

func TestMain_ConfigHelpFlagShowsSubcommandHelp(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"config", "--help"})
	assert.NoErr(t, err)
	assert.Eq(t, 0, len(calls))

	help := stdout.String() + stderr.String()
	if !strings.Contains(help, "init") || !strings.Contains(help, "get") || !strings.Contains(help, "set") {
		t.Fatalf("expected config subcommands in help output, got %q", help)
	}
}

func TestMain_ConfigPathHelpShowsSupportedTargets(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(func(string, any) error {
		t.Fatal("handler should not run")
		return nil
	}, &stdout, &stderr).RunWithArgs([]string{"cfg", "path", "--help"})

	assert.NoErr(t, err)
	help := stdout.String() + stderr.String()
	for _, target := range []string{
		"config_file",
		"config_dir",
		"env_file",
		"bin_dir",
		"cache_dir",
		"sdk_dir",
		"pkg_store_file",
		"sdk_store_file",
	} {
		assert.Contains(t, help, target)
	}
}

func TestMain_ConfigSubcommandRejectsUnknownFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(func(string, any) error { return nil }, &stdout, &stderr).
		RunWithArgs([]string{"config", "get", "--bad", "global.target"})
	if err == nil {
		t.Fatal("expected config get --bad to be rejected")
	}
	if !strings.Contains(err.Error(), "bad") {
		t.Fatalf("expected error to mention bad flag, got %v", err)
	}
}
