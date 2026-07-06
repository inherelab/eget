package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
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
	help := stdout.String()
	if !strings.Contains(help, "Usage") && !strings.Contains(help, "Commands") {
		t.Fatalf("expected help output to contain usage, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected stderr to be empty, got %q", stderr.String())
	}
}

func TestSetBuildInfoCompactsBuildTime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"rfc3339 offset", "2026-05-05T13:20:19+08:00", "2026-05-05T13:20:19"},
		{"makefile legacy", "2026/05/05-13:20:19", "2026-05-05T13:20:19"},
		{"unknown", "unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetBuildInfo("dev", "hash", tt.input)
			assert.Eq(t, tt.want, buildTime)
		})
	}
}

func TestBuildInfoReturnsConfiguredValues(t *testing.T) {
	SetBuildInfo("1.7.1", "abcdef12", "2026-05-25T10:20:30+08:00")

	info := BuildInfo()

	assert.Eq(t, "1.7.1", info.Version)
	assert.Eq(t, "abcdef12", info.GitHash)
	assert.Eq(t, "2026-05-25T10:20:30", info.BuildTime)
}

func TestMain_VersionUsesBuildInfo(t *testing.T) {
	SetBuildInfo("v1.2.3", "abc123", "2026-05-16T10:11:12+08:00")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := newApp(func(string, any) error {
		t.Fatalf("handler should not run for version")
		return nil
	}, &stdout, &stderr).RunWithArgs([]string{"--version"})

	assert.NoErr(t, err)
	out := stdout.String() + stderr.String()
	assert.Contains(t, out, "v1.2.3")
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

func TestMain_GlobalVerboseFlagAppearsInAppHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(nil, &stdout, &stderr)

	err := app.RunWithArgs([]string{"--help"})

	assert.NoErr(t, err)
	help := stdout.String() + stderr.String()
	assert.Contains(t, help, "--no-proxy")
	assert.Contains(t, help, "--verbose")
	assert.Contains(t, help, "-v")
}

func TestCommonCommandsIncludeHelpExamples(t *testing.T) {
	app := newApp(nil, io.Discard, io.Discard)
	for _, name := range []string{"install", "download", "add", "update", "list", "show", "uninstall"} {
		t.Run(name, func(t *testing.T) {
			cmd := app.inner.GetCommand(name)
			if cmd == nil {
				t.Fatalf("expected command %q", name)
			}
			assert.Contains(t, cmd.Help, "<info>Examples</>")
			assert.Contains(t, cmd.Help, "eget "+name)
		})
	}
}

func TestMain_GlobalVerboseFlagResetsBetweenRuns(t *testing.T) {
	calls := make([]commandCall, 0, 2)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(handler, &stdout, &stderr)
	assert.NoErr(t, app.RunWithArgs([]string{"-v", "install", "inhere/markview"}))
	assert.True(t, app.Verbose())

	assert.NoErr(t, app.RunWithArgs([]string{"install", "inhere/markview"}))

	assert.Eq(t, 2, len(calls))
	assert.False(t, app.Verbose())
}

func TestMain_GlobalNoProxyFlagParsesBeforeCommand(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(handler, &stdout, &stderr)
	err := app.RunWithArgs([]string{"--no-proxy", "install", "inhere/markview"})
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(calls))
	assert.True(t, app.NoProxy())
}

func TestMain_UninstallPurgeFlagParsesBeforeTarget(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(handler, &stdout, &stderr)
	err := app.RunWithArgs([]string{"rm", "--purge", "fzf"})
	if err != nil {
		t.Fatalf("run rm --purge: %v", err)
	}
	assert.Eq(t, 1, len(calls))

	opts, ok := calls[0].options.(*UninstallOptions)
	if !ok {
		t.Fatalf("expected uninstall options, got %T", calls[0].options)
	}
	assert.Eq(t, "uninstall", calls[0].name)
	assert.Eq(t, "fzf", opts.Target)
	assert.True(t, opts.Purge)
}

func TestMain_CommandFlagsParseAfterArguments(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		assert func(t *testing.T, call commandCall)
	}{
		{
			name: "download ghproxy",
			args: []string{"dl", "https://github.com/owner/repo/releases/download/v1.2.3/tool.zip", "--ghproxy"},
			assert: func(t *testing.T, call commandCall) {
				assert.Eq(t, "download", call.name)
				opts, ok := call.options.(*DownloadOptions)
				assert.True(t, ok)
				assert.Eq(t, "https://github.com/owner/repo/releases/download/v1.2.3/tool.zip", opts.Target)
				assert.True(t, opts.Ghproxy)
			},
		},
		{
			name: "add name",
			args: []string{"add", "owner/repo", "--name", "tool"},
			assert: func(t *testing.T, call commandCall) {
				assert.Eq(t, "add", call.name)
				opts, ok := call.options.(*AddOptions)
				assert.True(t, ok)
				assert.Eq(t, "owner/repo", opts.Target)
				assert.Eq(t, "tool", opts.Name)
			},
		},
		{
			name: "update tag",
			args: []string{"update", "tool", "--tag", "nightly"},
			assert: func(t *testing.T, call commandCall) {
				assert.Eq(t, "update", call.name)
				opts, ok := call.options.(*UpdateOptions)
				assert.True(t, ok)
				assert.Eq(t, []string{"tool"}, opts.Targets)
				assert.Eq(t, "nightly", opts.Tag)
			},
		},
		{
			name: "query action",
			args: []string{"query", "owner/repo", "--action", "assets"},
			assert: func(t *testing.T, call commandCall) {
				assert.Eq(t, "query", call.name)
				opts, ok := call.options.(*QueryOptions)
				assert.True(t, ok)
				assert.Eq(t, "owner/repo", opts.Target)
				assert.Eq(t, "assets", opts.Action)
			},
		},
		{
			name: "search json",
			args: []string{"search", "keyword", "--json"},
			assert: func(t *testing.T, call commandCall) {
				assert.Eq(t, "search", call.name)
				opts, ok := call.options.(*SearchOptions)
				assert.True(t, ok)
				assert.Eq(t, "keyword", opts.Keyword)
				assert.True(t, opts.JSON)
			},
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
			assert.Eq(t, 1, len(calls))
			tt.assert(t, calls[0])
		})
	}
}
