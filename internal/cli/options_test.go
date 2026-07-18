package cli

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
	"github.com/inherelab/eget/internal/install"
)

func TestNormalizeInstallMode(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{"p", install.InstallModePortable, false},
		{"port", install.InstallModePortable, false},
		{" portable ", install.InstallModePortable, false},
		{"P", install.InstallModePortable, false},
		{"i", install.InstallModeInstaller, false},
		{"ins", install.InstallModeInstaller, false},
		{"install", install.InstallModeInstaller, false},
		{" INSTALLER ", install.InstallModeInstaller, false},
		{"silent", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := normalizeInstallMode(tt.input)
			assert.Eq(t, tt.wantErr, err != nil)
			assert.Eq(t, tt.want, got)
		})
	}
}
