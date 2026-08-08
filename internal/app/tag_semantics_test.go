package app

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

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
