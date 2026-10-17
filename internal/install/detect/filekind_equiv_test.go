package detect

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

// TestEquivS_R01 pins classifyReleaseAsset (folded axis, base name) before it
// starts consuming internal/filekind, see
// docs/plans/2026-10-17-rule-consolidation-plan.md.
func TestEquivS_R01(t *testing.T) {
	cases := []struct {
		asset string
		want  releaseAssetKind
	}{
		{"tool_1.0_linux_amd64.tar.gz", releaseAssetInstallable},
		{"tool.exe", releaseAssetInstallable},

		{"releases", releaseAssetMetadata},
		{"RELEASES", releaseAssetMetadata},
		{"latest.yml", releaseAssetMetadata},
		{"latest-mac.yml", releaseAssetMetadata},
		{"latest-linux.yml", releaseAssetMetadata},
		{"latest.json", releaseAssetMetadata},
		{"tool.zip.blockmap", releaseAssetMetadata},
		{"tool.yml", releaseAssetMetadata},
		{"tool.yaml", releaseAssetMetadata},
		{"tool.sbom.json", releaseAssetMetadata},
		{"tool.spdx.json", releaseAssetMetadata},
		{"tool.cyclonedx.json", releaseAssetMetadata},
		{"tool.intoto", releaseAssetMetadata},
		{"tool.intoto.jsonl", releaseAssetMetadata},
		{"tool.provenance", releaseAssetMetadata},
		{"tool.attestation", releaseAssetMetadata},

		{"tool.zip.sha256", releaseAssetChecksum},
		{"tool.zip.sha256sum", releaseAssetChecksum},
		{"tool.zip.sha512", releaseAssetChecksum},
		{"tool.zip.sha512sum", releaseAssetChecksum},
		{"tool.zip.md5", releaseAssetChecksum},
		{"tool.zip.md5sum", releaseAssetChecksum},
		{"tool.checksums.txt", releaseAssetChecksum},
		{"tool.checksum.txt", releaseAssetChecksum},
		{"TOOL.ZIP.SHA256", releaseAssetChecksum},

		{"tool.zip.sig", releaseAssetSignature},
		{"tool.zip.asc", releaseAssetSignature},
		{"tool.zip.minisig", releaseAssetSignature},
		{"tool.zip.pem", releaseAssetSignature},
		{"tool.zip.crt", releaseAssetSignature},
		{"tool.zip.cert", releaseAssetSignature},
		{"tool.zip.gpg", releaseAssetSignature},
		{"tool.zip.sigstore", releaseAssetSignature},
		{"tool.zip.sigstore.json", releaseAssetSignature},
		{"TOOL.ZIP.SIG", releaseAssetSignature},

		{"releases/download/v1.0/tool.zip.sha256", releaseAssetChecksum},
	}
	for _, tc := range cases {
		t.Run(tc.asset, func(t *testing.T) {
			assert.Eq(t, tc.want, classifyReleaseAsset(tc.asset))
			assert.Eq(t, tc.want != releaseAssetInstallable, isReleaseMetadataAsset(tc.asset))
		})
	}

	t.Run("SelectableReleaseAssets drops metadata only", func(t *testing.T) {
		got := SelectableReleaseAssets([]string{"tool.tar.gz", "tool.zip.sha256", "latest.yml", "tool.zip.sig"})
		assert.Eq(t, []string{"tool.tar.gz"}, got)
	})
}
