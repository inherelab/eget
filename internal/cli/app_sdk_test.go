package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestMain_SDKConfigAddRoutesAndBindsOptions(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"sdk", "config", "add", "--mirror", "zulu", "--force", "java"})
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(calls))
	assert.Eq(t, "sdk.config.add", calls[0].name)
	opts, ok := calls[0].options.(*SDKConfigOptions)
	assert.True(t, ok)
	assert.Eq(t, "add", opts.Action)
	assert.Eq(t, "java", opts.Name)
	assert.Eq(t, "zulu", opts.Mirror)
	assert.True(t, opts.Force)
	assert.False(t, opts.All)
}

func TestMain_SDKPathRoutesAndBindsTarget(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"sdk", "path", "java:17"})
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(calls))
	assert.Eq(t, "sdk.path", calls[0].name)
	opts, ok := calls[0].options.(*SDKPathOptions)
	assert.True(t, ok)
	assert.Eq(t, "java:17", opts.Target)
}

func TestMain_SDKPathRejectsMissingTarget(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(func(string, any) error {
		t.Fatal("handler should not run")
		return nil
	}, &stdout, &stderr).RunWithArgs([]string{"sdk", "path"})
	assert.Err(t, err)
}

func TestMain_SDKConfigAddAllRoutesAndBindsOptions(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"sdk", "config", "add", "--all", "--mirror", "mirror"})
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(calls))
	opts, ok := calls[0].options.(*SDKConfigOptions)
	assert.True(t, ok)
	assert.True(t, opts.All)
	assert.Eq(t, "mirror", opts.Mirror)
	assert.Eq(t, "", opts.Name)
}

func TestMain_SDKConfigAddAllowsFlagsAfterName(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"sdk", "config", "add", "jdk", "--mirror", "huawei", "--force"})
	if err != nil {
		t.Fatalf("expected sdk config add to allow trailing flags, got %v", err)
	}
	assert.Eq(t, 1, len(calls))
	assert.Eq(t, "sdk.config.add", calls[0].name)
	opts, ok := calls[0].options.(*SDKConfigOptions)
	assert.True(t, ok)
	assert.Eq(t, "jdk", opts.Name)
	assert.Eq(t, "huawei", opts.Mirror)
	assert.True(t, opts.Force)
	assert.False(t, opts.All)
}

func TestMain_SDKConfigAddRejectsMirrorWithoutValue(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"before name", []string{"sdk", "config", "add", "--mirror", "--force", "java"}},
		{"after name", []string{"sdk", "config", "add", "jdk", "--mirror"}},
		{"empty value", []string{"sdk", "config", "add", "--mirror=", "java"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := newApp(func(string, any) error {
				t.Fatal("handler should not run")
				return nil
			}, &stdout, &stderr).RunWithArgs(tt.args)

			assert.Err(t, err)
			assert.Contains(t, err.Error(), "--mirror requires a value")
		})
	}
}

func TestMain_SDKConfigAddRejectsNameAndAll(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(func(string, any) error {
		t.Fatal("handler should not run")
		return nil
	}, &stdout, &stderr).RunWithArgs([]string{"sdk", "config", "add", "--all", "jdk"})

	assert.Err(t, err)
	assert.Contains(t, err.Error(), "requires exactly one")
}

func TestMain_SDKWithoutSubcommandShowsHelp(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"sdk"})
	assert.NoErr(t, err)
	assert.Eq(t, 0, len(calls))

	help := stdout.String()
	if !strings.Contains(help, "install") || !strings.Contains(help, "index") {
		t.Fatalf("expected sdk help output, got %q", help)
	}
}

func TestMain_SDKRoutesAndBindsOptions(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCmd    string
		assertOpts func(*testing.T, any)
	}{
		{
			name:    "install exact",
			args:    []string{"sdk", "install", "go@1.21.1"},
			wantCmd: "sdk.install",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKInstallOptions)
				assert.True(t, ok)
				assert.Eq(t, []string{"go@1.21.1"}, opts.Targets)
				assert.False(t, opts.Force)
			},
		},
		{
			name:    "install force multiple",
			args:    []string{"sdk", "install", "--force", "go@1.21.1", "node:20.11.1"},
			wantCmd: "sdk.install",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKInstallOptions)
				assert.True(t, ok)
				assert.Eq(t, []string{"go@1.21.1", "node:20.11.1"}, opts.Targets)
				assert.True(t, opts.Force)
			},
		},
		{
			name:    "download default",
			args:    []string{"sdk", "download", "go:1.22"},
			wantCmd: "sdk.download",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKDownloadOptions)
				assert.True(t, ok)
				assert.Eq(t, []string{"go:1.22"}, opts.Targets)
				assert.Eq(t, "", opts.OS)
				assert.Eq(t, "", opts.Arch)
				assert.Eq(t, "", opts.Output)
			},
		},
		{
			name:    "download alias platform output",
			args:    []string{"sdk", "dl", "--os", "windows", "--arch", "amd64", "-o", "downloads", "go:1.22", "node:20"},
			wantCmd: "sdk.download",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKDownloadOptions)
				assert.True(t, ok)
				assert.Eq(t, []string{"go:1.22", "node:20"}, opts.Targets)
				assert.Eq(t, "windows", opts.OS)
				assert.Eq(t, "amd64", opts.Arch)
				assert.Eq(t, "downloads", opts.Output)
			},
		},
		{
			name:    "list all",
			args:    []string{"sdk", "list"},
			wantCmd: "sdk.list",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKListOptions)
				assert.True(t, ok)
				assert.Eq(t, "", opts.Name)
				assert.False(t, opts.JSON)
			},
		},
		{
			name:    "list json name",
			args:    []string{"sdk", "list", "--json", "go"},
			wantCmd: "sdk.list",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKListOptions)
				assert.True(t, ok)
				assert.Eq(t, "go", opts.Name)
				assert.True(t, opts.JSON)
			},
		},
		{
			name:    "remove",
			args:    []string{"sdk", "remove", "go@1.21.1"},
			wantCmd: "sdk.remove",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKRemoveOptions)
				assert.True(t, ok)
				assert.Eq(t, "go@1.21.1", opts.Target)
			},
		},
		{
			name:    "search keywords",
			args:    []string{"sdk", "search", "go", "1.22", "amd64", "^windows"},
			wantCmd: "sdk.search",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKSearchOptions)
				assert.True(t, ok)
				assert.Eq(t, "go", opts.Name)
				assert.Eq(t, []string{"1.22", "amd64", "^windows"}, opts.Keywords)
				assert.Eq(t, 20, opts.Number)
				assert.False(t, opts.JSON)
			},
		},
		{
			name:    "search json",
			args:    []string{"sdk", "search", "--json", "--number", "5", "--sort", "desc", "node", "REG:^22"},
			wantCmd: "sdk.search",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKSearchOptions)
				assert.True(t, ok)
				assert.Eq(t, "node", opts.Name)
				assert.Eq(t, []string{"REG:^22"}, opts.Keywords)
				assert.Eq(t, 5, opts.Number)
				assert.Eq(t, "desc", opts.Sort)
				assert.True(t, opts.JSON)
			},
		},
		{
			name:    "search unlimited short number",
			args:    []string{"sdk", "search", "-n", "0", "node", "20"},
			wantCmd: "sdk.search",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKSearchOptions)
				assert.True(t, ok)
				assert.Eq(t, "node", opts.Name)
				assert.Eq(t, []string{"20"}, opts.Keywords)
				assert.Eq(t, 0, opts.Number)
			},
		},
		{
			name:    "search prerelease",
			args:    []string{"sdk", "search", "--prerelease", "go", "1.22"},
			wantCmd: "sdk.search",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKSearchOptions)
				assert.True(t, ok)
				assert.Eq(t, "go", opts.Name)
				assert.Eq(t, []string{"1.22"}, opts.Keywords)
				assert.True(t, opts.Prerelease)
			},
		},
		{
			name:    "index refresh name",
			args:    []string{"sdk", "index", "refresh", "go"},
			wantCmd: "sdk.index.refresh",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKIndexOptions)
				assert.True(t, ok)
				assert.Eq(t, "refresh", opts.Action)
				assert.Eq(t, "go", opts.Name)
			},
		},
		{
			name:    "index refresh all",
			args:    []string{"sdk", "index", "refresh", "--all"},
			wantCmd: "sdk.index.refresh",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKIndexOptions)
				assert.True(t, ok)
				assert.True(t, opts.All)
			},
		},
		{
			name:    "index refresh alias",
			args:    []string{"sdk", "idx", "build", "node"},
			wantCmd: "sdk.index.refresh",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKIndexOptions)
				assert.True(t, ok)
				assert.Eq(t, "refresh", opts.Action)
				assert.Eq(t, "node", opts.Name)
			},
		},
		{
			name:    "index list alias",
			args:    []string{"sdk", "idx", "ls"},
			wantCmd: "sdk.index.list",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKIndexOptions)
				assert.True(t, ok)
				assert.Eq(t, "list", opts.Action)
			},
		},
		{
			name:    "index clear all",
			args:    []string{"sdk", "index", "clear", "--all"},
			wantCmd: "sdk.index.clear",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKIndexOptions)
				assert.True(t, ok)
				assert.True(t, opts.All)
			},
		},
		{
			name:    "index show name",
			args:    []string{"sdk", "index", "show", "go"},
			wantCmd: "sdk.index.show",
			assertOpts: func(t *testing.T, options any) {
				opts, ok := options.(*SDKIndexOptions)
				assert.True(t, ok)
				assert.Eq(t, "show", opts.Action)
				assert.Eq(t, "go", opts.Name)
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
			assert.Eq(t, tt.wantCmd, calls[0].name)
			tt.assertOpts(t, calls[0].options)
		})
	}
}

func TestMain_SDKDownloadAllowsPartialPlatform(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantOS   string
		wantArch string
	}{
		{"os only", []string{"sdk", "download", "--os", "windows", "go:1.22"}, "windows", ""},
		{"arch only", []string{"sdk", "download", "--arch", "arm64", "go:1.22"}, "", "arm64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := make([]commandCall, 0, 1)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := newApp(func(name string, options any) error {
				calls = append(calls, commandCall{name: name, options: options})
				return nil
			}, &stdout, &stderr).RunWithArgs(tt.args)

			assert.NoErr(t, err)
			assert.Eq(t, 1, len(calls))
			opts, ok := calls[0].options.(*SDKDownloadOptions)
			assert.True(t, ok)
			assert.Eq(t, tt.wantOS, opts.OS)
			assert.Eq(t, tt.wantArch, opts.Arch)
		})
	}
}
