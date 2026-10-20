package cli

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
)

func TestApplyGlobalNetworkConfigDisableExternalToken(t *testing.T) {
	disabled := true
	cfg := cfgpkg.NewFile()
	cfg.Global.DisableExternalToken = &disabled

	opts := install.Options{}
	applyGlobalNetworkConfig(&opts, cfg)

	assert.True(t, opts.DisableExternalToken)
}

func TestApplyGlobalNetworkConfigDisableExternalTokenUnset(t *testing.T) {
	cfg := cfgpkg.NewFile()

	opts := install.Options{}
	applyGlobalNetworkConfig(&opts, cfg)

	assert.False(t, opts.DisableExternalToken)
}
