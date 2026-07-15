package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	cfgpkg "github.com/inherelab/eget/internal/config"
)

func TestHandleConfigDoctorPrintsLocalPaths(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", "")
	configPath := filepath.Join(tmp, "eget.toml")
	cacheDir := filepath.Join(tmp, "cache")
	targetDir := filepath.Join(tmp, "bin")
	sdkDir := filepath.Join(tmp, "sdks")
	assert.NoErr(t, os.MkdirAll(cacheDir, 0o755))
	assert.NoErr(t, os.MkdirAll(targetDir, 0o755))
	assert.NoErr(t, os.MkdirAll(sdkDir, 0o755))
	assert.NoErr(t, os.WriteFile(configPath, []byte("[global]\n"), 0o644))

	cacheValue := cacheDir
	targetValue := targetDir
	sdkValue := sdkDir
	proxyValue := "http://127.0.0.1:10801"
	tokenValue := "secret"
	cfg := cfgpkg.NewFile()
	cfg.Global.CacheDir = &cacheValue
	cfg.Global.Target = &targetValue
	cfg.Global.SDKTarget = &sdkValue
	cfg.Global.ProxyURL = &proxyValue
	cfg.Global.GithubToken = &tokenValue
	svc := &cliService{
		cfgService: app.ConfigService{
			ConfigPath: configPath,
			Load: func() (*cfgpkg.File, error) {
				return cfg, nil
			},
		},
		lookupEnv: func(key string) (string, bool) {
			switch key {
			case "EGET_CONFIG":
				return configPath, true
			case "EGET_CONFIG_DIR":
				return filepath.Dir(configPath), true
			case "EGET_BIN":
				return targetDir, true
			case "EGET_GITHUB_TOKEN":
				return "env-secret", true
			case "EGET_SELF_UPDATE_SOURCE":
				return "https://example.com/tools/eget/", true
			default:
				return "", false
			}
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleConfig(&ConfigOptions{Action: "doctor"})

	assert.NoErr(t, err)
	got := out.String()
	assert.Contains(t, got, "Eget config doctor result")
	assert.Contains(t, got, "[Config]")
	assert.Contains(t, got, "[Cache]")
	assert.Contains(t, got, "[Store]")
	assert.Contains(t, got, "[Runtime]")
	assert.Contains(t, got, "[Environment]")
	assert.Contains(t, got, configPath)
	assert.Contains(t, got, cacheDir)
	assert.Contains(t, got, filepath.Join(cacheDir, "pkg-cache"))
	assert.Contains(t, got, targetDir)
	assert.Contains(t, got, sdkDir)
	assert.Contains(t, got, filepath.Join(tmp, ".config", "eget", "installed.toml"))
	assert.Contains(t, got, filepath.Join(tmp, ".config", "eget", "sdk.installed.json"))
	assert.Contains(t, got, "github_token: set")
	assert.Contains(t, got, "EGET_CONFIG: "+configPath)
	assert.Contains(t, got, "EGET_CONFIG_DIR: "+filepath.Dir(configPath))
	assert.Contains(t, got, "EGET_BIN: "+targetDir)
	assert.Contains(t, got, "EGET_GITHUB_TOKEN: set")
	assert.Contains(t, got, "EGET_SELF_UPDATE_SOURCE: https://example.com/tools/eget/")
	assert.NotContains(t, got, "secret")
	assert.NotContains(t, got, "env-secret")
}

func TestHandleConfigExportStdoutIsPureTOML(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "eget.toml")
	writeCLIFile(t, configPath, "[global]\ntarget = '/machine/bin'\n\n[packages.fzf]\nrepo = 'junegunn/fzf'\n")
	svc := &cliService{cfgService: app.ConfigService{
		ConfigPath: configPath,
		Load:       func() (*cfgpkg.File, error) { return cfgpkg.LoadFile(configPath) },
	}}

	origStdout := os.Stdout
	reader, writer, err := os.Pipe()
	assert.NoErr(t, err)
	os.Stdout = writer
	defer func() { os.Stdout = origStdout }()

	err = svc.handleConfig(&ConfigOptions{Action: "export"})
	assert.NoErr(t, err)
	assert.NoErr(t, writer.Close())
	body, readErr := io.ReadAll(reader)
	assert.NoErr(t, readErr)
	assert.NoErr(t, reader.Close())
	assert.NotContains(t, string(body), "[global]")
	assert.Contains(t, string(body), "[packages.fzf]")
}

func TestHandleConfigExportWritesFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "eget.toml")
	exportPath := filepath.Join(dir, "portable.toml")
	writeCLIFile(t, configPath, "[global]\ntarget = '/machine/bin'\n\n[packages.fzf]\nrepo = 'junegunn/fzf'\n")
	svc := &cliService{cfgService: app.ConfigService{
		ConfigPath: configPath,
		Load:       func() (*cfgpkg.File, error) { return cfgpkg.LoadFile(configPath) },
	}}

	assert.NoErr(t, svc.handleConfig(&ConfigOptions{Action: "export", File: exportPath, WithGlobal: true}))
	body, err := os.ReadFile(exportPath)
	assert.NoErr(t, err)
	assert.Contains(t, string(body), "[global]")
}

func TestHandleConfigExportProtectsExistingFiles(t *testing.T) {
	t.Run("load error keeps existing output", func(t *testing.T) {
		exportPath := filepath.Join(t.TempDir(), "portable.toml")
		writeCLIFile(t, exportPath, "keep me")
		svc := &cliService{cfgService: app.ConfigService{Load: func() (*cfgpkg.File, error) {
			return nil, errors.New("load failed")
		}}}

		err := svc.handleConfig(&ConfigOptions{Action: "export", File: exportPath})

		assert.Err(t, err)
		body, readErr := os.ReadFile(exportPath)
		assert.NoErr(t, readErr)
		assert.Eq(t, "keep me", string(body))
	})

	t.Run("active config output is rejected", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "eget.toml")
		original := "[global]\ntarget = '/machine/bin'\n"
		writeCLIFile(t, configPath, original)
		svc := &cliService{cfgService: app.ConfigService{
			ConfigPath: configPath,
			Load:       func() (*cfgpkg.File, error) { return cfgpkg.LoadFile(configPath) },
		}}

		err := svc.handleConfig(&ConfigOptions{Action: "export", File: configPath})

		assert.Err(t, err)
		body, readErr := os.ReadFile(configPath)
		assert.NoErr(t, readErr)
		assert.Eq(t, original, string(body))
	})
}

func TestHandleConfigImportCanCancelOrForce(t *testing.T) {
	t.Run("cancel keeps target unchanged", func(t *testing.T) {
		dir := t.TempDir()
		targetPath := filepath.Join(dir, "eget.toml")
		sourcePath := filepath.Join(dir, "portable.toml")
		writeCLIFile(t, targetPath, "[global]\ntarget = '/current/bin'\n")
		writeCLIFile(t, sourcePath, "[packages.fzf]\nrepo = 'junegunn/fzf'\n")
		before, err := os.ReadFile(targetPath)
		assert.NoErr(t, err)
		svc := &cliService{cfgService: app.ConfigService{ConfigPath: targetPath}}
		withTestStdin(t, "n\n", func() {
			err = svc.handleConfig(&ConfigOptions{Action: "import", File: sourcePath})
		})
		assert.Err(t, err)
		after, readErr := os.ReadFile(targetPath)
		assert.NoErr(t, readErr)
		assert.Eq(t, before, after)
	})

	t.Run("force replaces without confirmation", func(t *testing.T) {
		dir := t.TempDir()
		targetPath := filepath.Join(dir, "eget.toml")
		sourcePath := filepath.Join(dir, "complete.toml")
		writeCLIFile(t, targetPath, "[global\ntarget =")
		writeCLIFile(t, sourcePath, "[global]\ntarget = '/imported/bin'\n")
		svc := &cliService{cfgService: app.ConfigService{ConfigPath: targetPath}}

		assert.NoErr(t, svc.handleConfig(&ConfigOptions{Action: "import", File: sourcePath, Force: true}))
		loaded, err := cfgpkg.LoadFile(targetPath)
		assert.NoErr(t, err)
		if err == nil {
			assert.Eq(t, "/imported/bin", *loaded.Global.Target)
		}
	})
}

func TestHandleConfigImportValidatesBeforeConfirmation(t *testing.T) {
	t.Run("malformed source returns parse error", func(t *testing.T) {
		dir := t.TempDir()
		targetPath := filepath.Join(dir, "eget.toml")
		sourcePath := filepath.Join(dir, "broken.toml")
		writeCLIFile(t, targetPath, "[global]\ntarget = '/current/bin'\n")
		writeCLIFile(t, sourcePath, "[global\ntarget =")
		svc := &cliService{cfgService: app.ConfigService{ConfigPath: targetPath}}
		var gotErr error
		withTestStdin(t, "", func() {
			gotErr = svc.handleConfig(&ConfigOptions{Action: "import", File: sourcePath})
		})
		assert.Err(t, gotErr)
		if gotErr != nil {
			assert.NotContains(t, gotErr.Error(), "cancelled")
			assert.Contains(t, gotErr.Error(), "toml")
		}
	})

	t.Run("same source and target returns same-file error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "eget.toml")
		writeCLIFile(t, path, "[global]\ntarget = '/current/bin'\n")
		svc := &cliService{cfgService: app.ConfigService{ConfigPath: path}}
		var gotErr error
		withTestStdin(t, "", func() {
			gotErr = svc.handleConfig(&ConfigOptions{Action: "import", File: path})
		})
		assert.Err(t, gotErr)
		if gotErr != nil {
			assert.Contains(t, gotErr.Error(), "same file")
		}
	})
}

func withTestStdin(t *testing.T, input string, fn func()) {
	t.Helper()
	original := os.Stdin
	reader, writer, err := os.Pipe()
	assert.NoErr(t, err)
	os.Stdin = reader
	defer func() { os.Stdin = original }()
	_, err = writer.WriteString(input)
	assert.NoErr(t, err)
	assert.NoErr(t, writer.Close())
	fn()
	assert.NoErr(t, reader.Close())
}

func TestHandleConfigDoctorKeepsConfigDirIndependentFromExplicitConfigFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("EGET_CONFIG_DIR", "")
	configPath := filepath.Join(tmp, "external", "eget.windows.toml")
	assert.NoErr(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	assert.NoErr(t, os.WriteFile(configPath, []byte("[global]\n"), 0o644))

	cfg := cfgpkg.NewFile()
	svc := &cliService{
		cfgService: app.ConfigService{
			ConfigPath: configPath,
			Load: func() (*cfgpkg.File, error) {
				return cfg, nil
			},
		},
		lookupEnv: func(key string) (string, bool) {
			if key == "EGET_CONFIG" {
				return configPath, true
			}
			return "", false
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleConfig(&ConfigOptions{Action: "doctor"})

	assert.NoErr(t, err)
	got := out.String()
	defaultConfigDir := filepath.Join(tmp, ".config", "eget")
	assert.Contains(t, got, "config_file: "+configPath)
	assert.Contains(t, got, "config_dir: "+defaultConfigDir)
	assert.Contains(t, got, "dotenv_file: "+filepath.Join(defaultConfigDir, ".env"))
	assert.NotContains(t, got, "config_dir: "+filepath.Dir(configPath))
}

func TestHandleConfigInitRejectsOverwriteWithoutConfirmation(t *testing.T) {
	svc := &cliService{
		cfgService: app.ConfigService{
			ConfigPath: "testdata/eget.toml",
			Load: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				target := "~/bin"
				cfg.Global.Target = &target
				return cfg, nil
			},
		},
	}

	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	if _, err := w.WriteString("n\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = w.Close()

	err = svc.handleConfig(&ConfigOptions{Action: "init"})
	if err == nil {
		t.Fatal("expected overwrite rejection error")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancellation error, got %v", err)
	}
}

func TestHandleConfigInitTreatsBlankOverwriteConfirmationAsCancel(t *testing.T) {
	svc := &cliService{
		cfgService: app.ConfigService{
			ConfigPath: "testdata/eget.toml",
			Load: func() (*cfgpkg.File, error) {
				return cfgpkg.NewFile(), nil
			},
		},
	}

	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	if _, err := w.WriteString("\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = w.Close()

	err = svc.handleConfig(&ConfigOptions{Action: "init"})
	if err == nil {
		t.Fatal("expected blank confirmation to cancel")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancellation error, got %v", err)
	}
}

func TestHandleConfigInitAllowsOverwriteWithConfirmation(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	svc := &cliService{
		cfgService: app.ConfigService{
			ConfigPath: configPath,
		},
	}

	if err := os.WriteFile(configPath, []byte("[global]\ntarget = \"~/bin\"\n"), 0o644); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	if _, err := w.WriteString("y\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = w.Close()

	if err := svc.handleConfig(&ConfigOptions{Action: "init"}); err != nil {
		t.Fatalf("expected overwrite confirmation to allow init, got %v", err)
	}

	cfg, err := cfgpkg.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Global.Target == nil || *cfg.Global.Target != "~/.local/bin" {
		t.Fatalf("expected config to be overwritten with defaults, got %#v", cfg.Global.Target)
	}
}

func TestHandleConfigPathPrintsPath(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")
	writeCLIFile(t, configPath, "[global]\ncache_dir = \"~/cache\"\n")
	svc := &cliService{
		cfgService: app.ConfigService{
			ConfigPath: configPath,
			Load: func() (*cfgpkg.File, error) {
				return cfgpkg.LoadFile(configPath)
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleConfig(&ConfigOptions{Action: "path", Target: "config_file"})
	assert.NoErr(t, err)
	assert.Eq(t, configPath+"\n", out.String())
}

func TestHandleConfigPathCheckPrintsExistsStatus(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")
	cacheDir := filepath.Join(tmp, "cache")
	writeCLIFile(t, configPath, "[global]\ncache_dir = \""+filepath.ToSlash(cacheDir)+"\"\n")
	assert.NoErr(t, os.MkdirAll(cacheDir, 0o755))
	svc := &cliService{
		cfgService: app.ConfigService{
			ConfigPath: configPath,
			Load: func() (*cfgpkg.File, error) {
				return cfgpkg.LoadFile(configPath)
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleConfig(&ConfigOptions{Action: "path", Target: "cache_dir", Check: true})
	assert.NoErr(t, err)
	assert.Eq(t, filepath.ToSlash(cacheDir)+", exists: true\n", filepath.ToSlash(out.String()))
}
