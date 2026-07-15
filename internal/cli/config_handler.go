package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/gookit/cliui/show"
	"github.com/gookit/cliui/show/lists"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/cli/prompts"
	cfgpkg "github.com/inherelab/eget/internal/config"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/sdk"
	"github.com/inherelab/eget/internal/util"
)

func (s *cliService) handleConfig(opts *ConfigOptions) error {
	switch opts.Action {
	case "init":
		info, err := s.cfgService.ConfigInfo()
		if err != nil {
			return err
		}
		if info.Exists {
			confirmed, err := prompts.ConfirmOverwrite(info.Path)
			if err != nil {
				return err
			}
			if !confirmed {
				return fmt.Errorf("config init cancelled")
			}
		}
		path, err := s.cfgService.ConfigInit()
		if err != nil {
			return err
		}
		ccolor.Successf("✓ Initialized config: %s\n", path)
		return nil
	case "list", "ls":
		info, err := s.cfgService.ConfigInfo()
		if err != nil {
			return err
		}
		cfg, err := s.cfgService.ConfigList()
		if err != nil {
			return err
		}

		showListConfig := func(opts *show.ListOptions) {
			opts.TagName = "toml"
			opts.FilterFunc = func(item *lists.Item) bool {
				switch item.RftVal().Kind() {
				case reflect.Map, reflect.Slice:
					if item.RftVal().IsNil() || item.RftVal().Len() == 0 {
						return false
					}
				}
				return true
			}
		}

		ccolor.Printf("# %s, exists: %v\n", info.Path, info.Exists)
		show.MList(map[string]any{
			"global":   cfg.Global,
			"apiCache": cfg.ApiCache,
			"ghproxy":  cfg.Ghproxy,
		}, showListConfig)

		// packages
		ccolor.Grayln("---------------------------")
		ccolor.Yellowln("📦 Configed Packages:")
		show.MList(cfg.Packages, showListConfig)

		// sdk
		ccolor.Grayln("---------------------------")
		ccolor.Yellowln("📦 Configed SDKs:")
		show.MList(cfg.SDK, showListConfig)
		return nil
	case "doctor":
		return s.handleConfigDoctor()
	case "export":
		if opts.File == "" {
			return s.cfgService.ConfigExport(os.Stdout, opts.WithGlobal)
		}
		info, err := s.cfgService.ConfigPathInfo("config_file")
		if err != nil {
			return err
		}
		if info.Exists {
			configInfo, err := os.Stat(info.Path)
			if err != nil {
				return err
			}
			if outputInfo, statErr := os.Stat(opts.File); statErr == nil {
				if os.SameFile(configInfo, outputInfo) {
					return fmt.Errorf("config export target is the active config file: %s", opts.File)
				}
			} else if !os.IsNotExist(statErr) {
				return statErr
			}
		}
		var out bytes.Buffer
		if err := s.cfgService.ConfigExport(&out, opts.WithGlobal); err != nil {
			return err
		}
		if err := os.WriteFile(opts.File, out.Bytes(), 0o644); err != nil {
			return err
		}
		ccolor.Successf("✓ Exported config: %s\n", opts.File)
		return nil
	case "import":
		info, err := s.cfgService.ConfigPathInfo("config_file")
		if err != nil {
			return err
		}
		if err := s.cfgService.ConfigImportCheck(opts.File); err != nil {
			return err
		}
		if info.Exists && !opts.Force {
			confirmed, err := prompts.ConfirmOverwrite(info.Path)
			if err != nil {
				return err
			}
			if !confirmed {
				return fmt.Errorf("config import cancelled")
			}
		}
		path, err := s.cfgService.ConfigImport(opts.File)
		if err != nil {
			return err
		}
		ccolor.Successf("✓ Imported config: %s\n", path)
		return nil
	case "path":
		info, err := s.cfgService.ConfigPathInfo(opts.Target)
		if err != nil {
			return err
		}
		if opts.Check {
			ccolor.Printf("%s, exists: %v\n", info.Path, info.Exists)
			return nil
		}
		ccolor.Println(info.Path)
		return nil
	case "get":
		value, err := s.cfgService.ConfigGet(opts.Key)
		if err != nil {
			return err
		}
		if value == nil {
			ccolor.Infoln("nil")
		} else if str, ok := value.(string); ok {
			ccolor.Infoln(str)
		} else {
			show.JSON(value)
		}
		return nil
	case "set":
		err := s.cfgService.ConfigSet(opts.Key, opts.Value)
		if err == nil {
			ccolor.Successf("✓ Set config: %s = %s\n", opts.Key, opts.Value)
		}
		return err
	default:
		return fmt.Errorf("config action is required")
	}
}

func (s *cliService) handleConfigDoctor() error {
	info, err := s.cfgService.ConfigInfo()
	if err != nil {
		return err
	}
	cfg, err := s.cfgService.ConfigList()
	if err != nil {
		return err
	}

	configPath := info.Path
	if configPath == "" {
		configPath = s.cfgService.ConfigPath
	}
	if configPath == "" {
		if writable, err := cfgpkg.ResolveWritablePath(); err == nil {
			configPath = writable
		}
	}
	configDir := s.doctorConfigDir(configPath)
	cacheDir := util.ExpandPathOrRaw(util.FirstNonEmptyString(util.DerefString(cfg.Global.CacheDir), "~/.cache/eget"))
	targetDir := util.ExpandPathOrRaw(util.FirstNonEmptyString(util.DerefString(cfg.Global.Target), "~/.local/bin"))
	sdkTargetDir := util.ExpandPathOrRaw(util.FirstNonEmptyString(util.DerefString(cfg.Global.SDKTarget), "~/.local/sdks"))
	dotenvPath := filepath.Join(configDir, ".env")
	installedPath := resolveInstalledStorePath()
	sdkInstalledPath := resolveSDKInstalledStorePath()

	ccolor.Infoln("📇 Eget config doctor result")
	printDoctorSection("Config")
	printDoctorPath("config_file", configPath, info.Exists)
	printDoctorPath("config_dir", configDir, util.DirExists(configDir))
	printDoctorPath("dotenv_file", dotenvPath, util.FileExists(dotenvPath))
	printDoctorSection("Store")
	printDoctorPath("installed_store", installedPath, util.FileExists(installedPath))
	printDoctorPath("sdk_installed_store", sdkInstalledPath, util.FileExists(sdkInstalledPath))
	printDoctorSection("Cache")
	printDoctorPath("cache_dir", cacheDir, util.DirExists(cacheDir))
	printDoctorPath("pkg_cache_dir", filepath.Join(cacheDir, "pkg-cache"), util.DirExists(filepath.Join(cacheDir, "pkg-cache")))
	printDoctorPath("api_cache_dir", filepath.Join(cacheDir, "api-cache"), util.DirExists(filepath.Join(cacheDir, "api-cache")))
	printDoctorPath("sdk_index_dir", filepath.Join(cacheDir, "sdk-index"), util.DirExists(filepath.Join(cacheDir, "sdk-index")))
	printDoctorSection("Runtime")
	printDoctorPath("target_dir", targetDir, util.DirExists(targetDir))
	printDoctorPath("sdk_target_dir", sdkTargetDir, util.DirExists(sdkTargetDir))
	if guiTarget := strings.TrimSpace(util.DerefString(cfg.Global.GuiTarget)); guiTarget != "" {
		path := util.ExpandPathOrRaw(guiTarget)
		printDoctorPath("gui_target_dir", path, util.DirExists(path))
	}
	if sys7zPath := strings.TrimSpace(util.DerefString(cfg.Global.Sys7zPath)); sys7zPath != "" {
		path := util.ExpandPathOrRaw(sys7zPath)
		printDoctorPath("sys7z_path", path, util.FileExists(path))
	}
	ccolor.Printf("proxy_url: %s\n", setStatus(util.DerefString(cfg.Global.ProxyURL)))
	ccolor.Printf("github_token: %s\n", setStatus(util.DerefString(cfg.Global.GithubToken)))
	ccolor.Printf("cache_dir_writable: %v\n", dirWritable(cacheDir))
	ccolor.Printf("target_dir_writable: %v\n", dirWritable(targetDir))
	ccolor.Printf("sdk_target_dir_writable: %v\n", dirWritable(sdkTargetDir))
	printDoctorSection("Environment")
	s.printDoctorEnv()
	return nil
}

func printDoctorSection(name string) {
	ccolor.Warnf("\n[%s]\n", name)
}

func printDoctorPath(name, path string, exists bool) {
	ccolor.Printf("%s: %s (exists: <green>%v</>)\n", name, path, exists)
}

func (s *cliService) doctorConfigDir(configPath string) string {
	lookupEnv := os.LookupEnv
	if s != nil && s.lookupEnv != nil {
		lookupEnv = s.lookupEnv
	}
	homeDir, err := util.Home()
	if err != nil {
		return filepath.Dir(configPath)
	}
	return filepath.Dir(cfgpkg.OSConfigPath(homeDir, runtime.GOOS, lookupEnv))
}

func (s *cliService) printDoctorEnv() {
	lookupEnv := os.LookupEnv
	if s != nil && s.lookupEnv != nil {
		lookupEnv = s.lookupEnv
	}
	for _, item := range []struct {
		name   string
		secret bool
	}{
		{name: "EGET_CONFIG"},
		{name: "EGET_CONFIG_DIR"},
		{name: "EGET_BIN"},
		{name: "EGET_GITHUB_TOKEN", secret: true},
		{name: "EGET_SELF_UPDATE_SOURCE"},
	} {
		value, ok := lookupEnv(item.name)
		if !ok || value == "" {
			ccolor.Printf("%s: <unset>\n", item.name)
			continue
		}
		if item.secret {
			ccolor.Printf("%s: set\n", item.name)
			continue
		}
		ccolor.Printf("%s: %s\n", item.name, value)
	}
}

func resolveInstalledStorePath() string {
	store, err := storepkg.DefaultStore()
	if err != nil {
		return ""
	}
	return store.Path()
}

func resolveSDKInstalledStorePath() string {
	path, err := sdk.DefaultStorePath()
	if err != nil {
		return ""
	}
	return path
}

func setStatus(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unset"
	}
	return "set"
}

func dirWritable(path string) bool {
	if !util.DirExists(path) {
		return false
	}
	probe, err := os.CreateTemp(path, ".eget-doctor-*")
	if err != nil {
		return false
	}
	name := probe.Name()
	closeErr := probe.Close()
	removeErr := os.Remove(name)
	return closeErr == nil && removeErr == nil
}
