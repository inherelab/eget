package app

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestTrackingTagIgnoresVersionTags(t *testing.T) {
	tests := map[string]string{
		"v3.2.5":         "",
		"3.2.5":          "",
		"release-v1.2.3": "",
		"nightly":        "nightly",
		"latest-dev":     "latest-dev",
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			assert.Eq(t, expected, trackingTag(input))
		})
	}
}
