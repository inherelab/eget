package config

import (
	"os"
	"runtime"
)

func Load() (*File, error) {
	path, err := ResolveConfigPath()
	if err != nil {
		if IsNotExist(err) {
			return NewFile(), nil
		}
		return nil, err
	}

	file, err := LoadFile(path)
	if err != nil {
		if IsNotExist(err) {
			return NewFile(), nil
		}
		return nil, err
	}

	return file, nil
}

func LoadFile(path string) (*File, error) {
	cfg, err := loadConfigManager(path)
	if err != nil {
		return nil, err
	}
	decoded, err := decodeConfigFile(cfg)
	if err != nil {
		return nil, err
	}
	return applyPlatformDefaults(decoded, runtime.GOOS), nil
}

func NewFile() *File {
	cfg := &File{}
	apiCacheEnabled := true
	cfg.ApiCache.Enable = &apiCacheEnabled
	cfg.Repos = make(map[string]Section)
	cfg.Packages = make(map[string]Section)
	cfg.PkgTemplates = make(map[string]Section)
	cfg.SDK = make(map[string]SDKSection)
	cfg.Ext = make(map[string]ManagerSection)
	cfg.Global.SDKExtMap = defaultSDKExtMap()
	return applyPlatformDefaults(cfg, runtime.GOOS)
}

// defaultGuiTarget is the platform default of global.gui_target: macOS installs
// portable GUI applications as *.app bundles into ~/Applications, where Launchpad
// and Spotlight look for them; every other platform keeps no default.
func defaultGuiTarget(goos string) string {
	if goos == "darwin" {
		return "~/Applications"
	}
	return ""
}

// applyPlatformDefaults fills the defaults that are not spelled out in the file.
// An explicit global.gui_target always wins.
func applyPlatformDefaults(cfg *File, goos string) *File {
	if cfg == nil {
		return cfg
	}
	if cfg.Global.GuiTarget == nil {
		if target := defaultGuiTarget(goos); target != "" {
			cfg.Global.GuiTarget = &target
		}
	}
	return cfg
}

func defaultSDKExtMap() map[string]string {
	return map[string]string{
		"windows": "zip",
		"linux":   "tar.gz",
		"darwin":  "tar.gz",
	}
}

func LoadFromEnvOrDefault(lookupEnv func(string) (string, bool), homeDir, goos string) (*File, string, error) {
	path, err := resolveConfigPath(pathOptions{
		HomeDir:   homeDir,
		GOOS:      goos,
		LookupEnv: lookupEnv,
	})
	if err != nil {
		if os.IsNotExist(err) {
			return NewFile(), "", nil
		}
		return nil, "", err
	}

	cfg, err := LoadFile(path)
	if err != nil {
		return nil, "", err
	}

	return cfg, path, nil
}
