package app

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

// TestEquivS_F05 pins the tag-policy side of the filter migration
// (docs/plans/2026-10-17-rule-consolidation-plan.md): the caller trims before
// asking, and "^" stays literal for tags.
func TestEquivS_F05(t *testing.T) {
	t.Run("explicit tag policy", func(t *testing.T) {
		assert.Eq(t, tagPolicyPattern, tagPolicyForInstall("PRE:v1", tagPolicyTag))
		assert.Eq(t, tagPolicyPattern, tagPolicyForInstall(" PRE:v1", tagPolicyTag))
		assert.Eq(t, tagPolicyPattern, tagPolicyForInstall("PRE:v1 ", tagPolicyTag))
		assert.Eq(t, tagPolicyTag, tagPolicyForInstall("^PRE:v1", tagPolicyTag))
		assert.Eq(t, tagPolicyTag, tagPolicyForInstall("v1.2.3", tagPolicyTag))
	})

	t.Run("inferred tag policy", func(t *testing.T) {
		assert.Eq(t, tagPolicyPattern, tagPolicyForInstall("REG:^v1", ""))
		assert.Eq(t, tagPolicyPattern, tagPolicyForInstall("SUF:-linux", ""))
		assert.Eq(t, tagPolicyLatest, tagPolicyForInstall("v1.2.3", ""))
		assert.Eq(t, tagPolicyLatest, tagPolicyForInstall("", ""))
	})
}

func TestTrackingTagIgnoresVersionTags(t *testing.T) {
	tests := map[string]string{
		"v3.2.5":                        "",
		"3.2.5":                         "",
		"release-v1.2.3":                "",
		"@moonshot-ai/kimi-code@0.28.1": "",
		"nightly":                       "nightly",
		"latest-dev":                    "latest-dev",
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			assert.Eq(t, expected, trackingTag(input))
		})
	}
}

func TestTrackingTagPolicyOverridesHeuristic(t *testing.T) {
	tests := map[string]string{
		"latest": "",
		"tag":    "v3.2.5",
		"":       "",
		"bad":    "",
	}

	for policy, expected := range tests {
		t.Run(policy, func(t *testing.T) {
			assert.Eq(t, expected, trackingTagWithPolicy("v3.2.5", policy))
		})
	}
}

func TestTagPolicyForInstallDefaultsToLatestWithoutExplicitTag(t *testing.T) {
	assert.Eq(t, tagPolicyLatest, tagPolicyForInstall("", ""))
}

func TestTagPolicyForInstallRecognisesPatterns(t *testing.T) {
	assert.Eq(t, tagPolicyPattern, tagPolicyForInstall("PRE:cua-driver-rs-v", ""))
	assert.Eq(t, tagPolicyPattern, tagPolicyForInstall("SUF:-nightly", ""))
	assert.Eq(t, tagPolicyPattern, tagPolicyForInstall(`REG:^app-v\d+`, ""))
	// --track-tag on a pattern still resolves to the pattern policy, so update
	// re-resolves the newest match instead of pinning one exact tag.
	assert.Eq(t, tagPolicyPattern, tagPolicyForInstall("PRE:cua-driver-rs-v", "tag"))

	// plain tags keep the existing behaviour
	assert.Eq(t, tagPolicyTag, tagPolicyForInstall("nightly", ""))
	assert.Eq(t, tagPolicyLatest, tagPolicyForInstall("v3.2.5", ""))
}

func TestTrackingTagKeepsPatternPolicy(t *testing.T) {
	assert.Eq(t, "PRE:cua-driver-rs-v", trackingTagWithPolicy("PRE:cua-driver-rs-v", tagPolicyPattern))
	assert.Eq(t, "", trackingTagWithPolicy("PRE:cua-driver-rs-v", tagPolicyLatest))
}

func TestCleanTagPolicyAcceptsPattern(t *testing.T) {
	assert.Eq(t, tagPolicyPattern, cleanTagPolicy("pattern"))
	assert.Eq(t, "", cleanTagPolicy("nonsense"))
}
