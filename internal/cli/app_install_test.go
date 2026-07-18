package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
	"github.com/inherelab/eget/internal/install"
)

func TestMain_InstallStandardOrderRoutesAndBindsOptions(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"install", "--tag", "nightly", "--rename", "tool-linux-amd64=tool", "inhere/markview"})
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
	if opts.Rename != "tool-linux-amd64=tool" {
		t.Fatalf("expected rename option to bind, got %q", opts.Rename)
	}
	if len(opts.Targets) != 1 || opts.Targets[0] != "inhere/markview" {
		t.Fatalf("expected target inhere/markview, got %#v", opts.Targets)
	}
}

func TestMain_DownloadGhproxyFlagBindsOption(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"dl", "--ghproxy", "https://github.com/owner/repo/releases/download/v1.2.3/tool.zip"})
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(calls))
	assert.Eq(t, "download", calls[0].name)
	opts, ok := calls[0].options.(*DownloadOptions)
	assert.True(t, ok)
	assert.True(t, opts.Ghproxy)
	assert.Eq(t, "https://github.com/owner/repo/releases/download/v1.2.3/tool.zip", opts.Target)
}

func TestMain_InstallBindsMultipleTargets(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"space separated", []string{"install", "fzf", "rg"}, []string{"fzf", "rg"}},
		{"comma separated", []string{"install", "fzf,rg,fd"}, []string{"fzf", "rg", "fd"}},
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
			opts, ok := calls[0].options.(*InstallOptions)
			assert.True(t, ok)
			assert.Eq(t, tt.want, opts.Targets)
		})
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

func TestMain_StripComponentsFlagBindsInstallDownloadAndAdd(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"install", []string{"install", "--extract-all", "--strip-components", "1", "inhere/markview"}, "install"},
		{"download", []string{"download", "--extract-all", "--strip-components", "1", "inhere/markview"}, "download"},
		{"add", []string{"add", "--extract-all", "--strip-components", "1", "inhere/markview"}, "add"},
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
			assert.Eq(t, tt.want, calls[0].name)
			switch opts := calls[0].options.(type) {
			case *InstallOptions:
				assert.Eq(t, 1, opts.StripComponents)
			case *DownloadOptions:
				assert.Eq(t, 1, opts.StripComponents)
			case *AddOptions:
				assert.Eq(t, 1, opts.StripComponents)
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
		{"install alias gui", []string{"ins", "--gui", "inhere/markview"}, "install"},
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

func TestMain_InstallModeFlagBindsInstallAlias(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"ins", "--gui", "--install-mode", "installer", "owner/repo"})
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(calls))
	assert.Eq(t, "install", calls[0].name)
	opts, ok := calls[0].options.(*InstallOptions)
	assert.True(t, ok)
	assert.True(t, opts.GUI)
	assert.Eq(t, "installer", opts.InstallMode)
	assert.Eq(t, []string{"owner/repo"}, opts.Targets)
}

func TestMain_InstallModeNormalizesGUI(t *testing.T) {
	tests := []struct {
		name, mode, wantMode string
		gui, wantGUI         bool
	}{
		{"gui defaults installer", "", install.InstallModeInstaller, true, true},
		{"portable alias enables gui", "p", install.InstallModePortable, false, true},
		{"portable short name enables gui", "port", install.InstallModePortable, false, true},
		{"installer alias enables gui", "ins", install.InstallModeInstaller, false, true},
		{"install short name enables gui", "install", install.InstallModeInstaller, false, true},
		{"explicit portable beats gui default", "portable", install.InstallModePortable, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *InstallOptions
			handler := func(_ string, options any) error {
				got = options.(*InstallOptions)
				return nil
			}
			args := []string{"install"}
			if tt.gui {
				args = append(args, "--gui")
			}
			if tt.mode != "" {
				args = append(args, "--install-mode", tt.mode)
			}
			args = append(args, "owner/repo")
			var stdout, stderr bytes.Buffer
			err := newApp(handler, &stdout, &stderr).RunWithArgs(args)
			assert.NoErr(t, err)
			assert.True(t, got != nil)
			assert.Eq(t, tt.wantGUI, got.GUI)
			assert.Eq(t, tt.wantMode, got.InstallMode)
		})
	}
}

func TestMain_InstallModeNormalizationResets(t *testing.T) {
	var got []*InstallOptions
	handler := func(_ string, options any) error {
		got = append(got, options.(*InstallOptions))
		return nil
	}
	var stdout, stderr bytes.Buffer
	app := newApp(handler, &stdout, &stderr)
	assert.NoErr(t, app.RunWithArgs([]string{"install", "--install-mode", "p", "owner/repo"}))
	assert.NoErr(t, app.RunWithArgs([]string{"install", "owner/repo"}))
	assert.True(t, got[0].GUI)
	assert.Eq(t, install.InstallModePortable, got[0].InstallMode)
	assert.False(t, got[1].GUI)
	assert.Eq(t, "", got[1].InstallMode)
}

func TestMain_InstallModeShortAliasNormalizes(t *testing.T) {
	var got *InstallOptions
	handler := func(_ string, options any) error {
		got = options.(*InstallOptions)
		return nil
	}
	var stdout, stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"install", "-imode", "port", "owner/repo"})
	assert.NoErr(t, err)
	if err != nil {
		return
	}
	assert.True(t, got != nil)
	assert.True(t, got.GUI)
	assert.Eq(t, install.InstallModePortable, got.InstallMode)
}

func TestMain_InstallRejectsInvalidInstallMode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(func(string, any) error {
		t.Fatal("handler should not run")
		return nil
	}, &stdout, &stderr).RunWithArgs([]string{"install", "--gui", "--install-mode", "silent", "owner/repo"})

	assert.Err(t, err)
	assert.Contains(t, err.Error(), "install mode")
}

func TestMain_FallbackVersionsFlagBindsInstallAndDownload(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"install", []string{"install", "--fallback-versions", "10", "sourceforge:keepass/Translations 2.x"}, "install"},
		{"download", []string{"download", "--fallback-versions", "10", "sourceforge:keepass/Translations 2.x"}, "download"},
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
				if opts.FallbackVersions != 10 {
					t.Fatalf("expected install fallback versions 10, got %d", opts.FallbackVersions)
				}
			case *DownloadOptions:
				if opts.FallbackVersions != 10 {
					t.Fatalf("expected download fallback versions 10, got %d", opts.FallbackVersions)
				}
			default:
				t.Fatalf("unexpected options type %T", calls[0].options)
			}
		})
	}
}

func TestMain_PrereleaseFlagBindsInstallAndDownload(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"install", []string{"install", "--prerelease", "DarkHobbit/doublecontact"}, "install"},
		{"download", []string{"download", "--prerelease", "DarkHobbit/doublecontact"}, "download"},
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
			assert.Eq(t, tt.want, calls[0].name)
			switch opts := calls[0].options.(type) {
			case *InstallOptions:
				assert.True(t, opts.Prerelease)
			case *DownloadOptions:
				assert.True(t, opts.Prerelease)
			default:
				t.Fatalf("unexpected options type %T", calls[0].options)
			}
		})
	}
}

func TestMain_ConcurrencyFlagsBindInstallDownloadUpdateAndAdd(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"install chunk", []string{"install", "--chunk", "8", "owner/repo"}, "install"},
		{"download chunk", []string{"download", "--chunk", "8", "owner/repo"}, "download"},
		{"update chunk", []string{"update", "--chunk", "8", "fd"}, "update"},
		{"add chunk", []string{"add", "--chunk", "3", "sharkdp/fd"}, "add"},
		{"install all batch", []string{"install", "--all", "--batch", "3"}, "install"},
		{"update all batch", []string{"update", "--all", "--batch", "3"}, "update"},
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
				if strings.Contains(tt.name, "chunk") && opts.ChunkConcurrency != 8 {
					t.Fatalf("expected install chunk=8, got %d", opts.ChunkConcurrency)
				}
				if strings.Contains(tt.name, "batch") && opts.BatchConcurrency != 3 {
					t.Fatalf("expected install batch=3, got %d", opts.BatchConcurrency)
				}
			case *DownloadOptions:
				if opts.ChunkConcurrency != 8 {
					t.Fatalf("expected download chunk=8, got %d", opts.ChunkConcurrency)
				}
			case *UpdateOptions:
				if strings.Contains(tt.name, "chunk") && opts.ChunkConcurrency != 8 {
					t.Fatalf("expected update chunk=8, got %d", opts.ChunkConcurrency)
				}
				if strings.Contains(tt.name, "batch") && opts.BatchConcurrency != 3 {
					t.Fatalf("expected update batch=3, got %d", opts.BatchConcurrency)
				}
			case *AddOptions:
				if opts.ChunkConcurrency != 3 {
					t.Fatalf("expected add chunk=3, got %d", opts.ChunkConcurrency)
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

func TestMain_InstallAllFlagBindsWithoutTarget(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"install", "--all"})
	if err != nil {
		t.Fatalf("expected install --all to parse, got %v", err)
	}
	if len(calls) != 1 || calls[0].name != "install" {
		t.Fatalf("unexpected routed call: %#v", calls)
	}
	opts, ok := calls[0].options.(*InstallOptions)
	if !ok {
		t.Fatalf("expected InstallOptions, got %T", calls[0].options)
	}
	if !opts.InstallAll {
		t.Fatalf("expected install all flag to be true")
	}
	if len(opts.Targets) != 0 {
		t.Fatalf("expected install --all to omit target, got %#v", opts.Targets)
	}
}

func TestMain_DownloadAndAddRejectRemovedAllFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
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

func TestMain_InstallBindsFlagsAfterTarget(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"install", "inhere/markview", "--tag", "nightly"})

	assert.NoErr(t, err)
	assert.Eq(t, 1, len(calls))
	opts, ok := calls[0].options.(*InstallOptions)
	assert.True(t, ok)
	assert.Eq(t, []string{"inhere/markview"}, opts.Targets)
	assert.Eq(t, "nightly", opts.Tag)
}

func TestMain_InstallTrackTagFlagParses(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"install", "AandStep/ResultV", "--tag", "v3.2.5", "--track-tag"})

	assert.NoErr(t, err)
	assert.Eq(t, 1, len(calls))
	opts, ok := calls[0].options.(*InstallOptions)
	assert.True(t, ok)
	assert.True(t, opts.TrackTag)
}
