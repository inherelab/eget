package cli

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
	"github.com/inherelab/eget/internal/app"
)

func TestUpdateDoneLine(t *testing.T) {
	tests := []struct {
		name   string
		result app.UpdatePackageResult
		want   string
	}{
		{
			name:   "managed package moved",
			result: app.UpdatePackageResult{Name: "fd", InstalledTag: "v1.0.0", LatestTag: "v10.0.0", Updated: true},
			want:   "updated fd v1.0.0 -> v10.0.0",
		},
		{
			name: "external package is named manager:package",
			result: app.UpdatePackageResult{
				Name: "typescript", Target: "npm:typescript", Manager: "npm",
				InstalledTag: "5.8.0", LatestTag: "5.9.2", Updated: true,
			},
			want: "updated npm:typescript 5.8.0 -> 5.9.2",
		},
		{
			name:   "update without a version move",
			result: app.UpdatePackageResult{Name: "fd", Updated: true},
			want:   "updated fd",
		},
		{
			name:   "nothing to do",
			result: app.UpdatePackageResult{Name: "fd", InstalledTag: "v10.0.0"},
			want:   "fd is already up to date: v10.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Eq(t, tt.want, updateDoneLine(tt.result))
		})
	}
}
