package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestDefaultGuiTargetByGOOS(t *testing.T) {
	assert.Eq(t, "~/Applications", defaultGuiTarget("darwin"))
	assert.Eq(t, "", defaultGuiTarget("windows"))
	assert.Eq(t, "", defaultGuiTarget("linux"))
}

func TestApplyPlatformDefaultsSetsDarwinGuiTarget(t *testing.T) {
	cfg := applyPlatformDefaults(&File{}, "darwin")
	assert.NotNil(t, cfg.Global.GuiTarget)
	assert.Eq(t, "~/Applications", *cfg.Global.GuiTarget)
}

func TestApplyPlatformDefaultsKeepsExplicitGuiTarget(t *testing.T) {
	explicit := "~/Apps"
	cfg := applyPlatformDefaults(&File{Global: Section{GuiTarget: &explicit}}, "darwin")
	assert.Eq(t, "~/Apps", *cfg.Global.GuiTarget)

	cfg = applyPlatformDefaults(&File{}, "linux")
	assert.Nil(t, cfg.Global.GuiTarget)
}

func TestLoadFileAppliesPlatformGuiTargetDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "eget.toml")
	if err := os.WriteFile(path, []byte("[global]\ntarget = \"~/bin\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFile(path)
	assert.NoErr(t, err)

	if defaultGuiTarget(runtime.GOOS) == "" {
		assert.Nil(t, cfg.Global.GuiTarget)
		return
	}
	assert.NotNil(t, cfg.Global.GuiTarget)
	assert.Eq(t, "~/Applications", *cfg.Global.GuiTarget)
}
