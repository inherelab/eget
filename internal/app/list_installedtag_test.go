package app

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
	storepkg "github.com/inherelab/eget/internal/installed"
)

func TestListOutdatedPackagesPassesInstalledTag(t *testing.T) {
	// the installed tag rides along on the check target so the GitHub fast
	// path can settle "family max == installed" without scanning releases
	var got LatestCheckTarget
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"trycua/cua": {
					Repo: "trycua/cua",
					Tag:  "cua-driver-rs-v0.35.4",
					Options: map[string]any{
						"tag":        "PRE:cua-driver-rs-v",
						"tag_policy": "pattern",
					},
				},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			got = target
			return LatestInfo{Tag: "cua-driver-rs-v0.35.4"}, nil
		},
	}

	items, failures, _, err := svc.ListOutdatedPackages()

	assert.NoErr(t, err)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 0, len(items))
	assert.Eq(t, "cua-driver-rs-v0.35.4", got.InstalledTag)
	assert.Eq(t, "PRE:cua-driver-rs-v", got.Tag)
}
