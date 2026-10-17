package filekind

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestClassifyAsset(t *testing.T) {
	t.Run("installable", func(t *testing.T) {
		for _, name := range []string{
			"tool_1.0_linux_amd64.tar.gz",
			"tool_1.0_windows_amd64.zip",
			"tool.exe",
		} {
			assert.Eq(t, AssetInstallable, ClassifyAsset(name), name)
		}
	})

	t.Run("metadata", func(t *testing.T) {
		for _, name := range []string{
			"releases",
			"RELEASES",
			"latest.yml",
			"latest-mac.yml",
			"latest-linux.yml",
			"latest.json",
			"tool.zip.blockmap",
			"tool.yml",
			"tool.yaml",
			"tool.sbom.json",
			"tool.spdx.json",
			"tool.cyclonedx.json",
			"tool.intoto",
			"tool.intoto.jsonl",
			"tool.provenance",
			"tool.attestation",
		} {
			assert.Eq(t, AssetMetadata, ClassifyAsset(name), name)
		}
	})

	t.Run("checksum", func(t *testing.T) {
		for _, name := range []string{
			"tool.zip.sha256",
			"tool.zip.sha256sum",
			"tool.zip.sha512",
			"tool.zip.sha512sum",
			"tool.zip.md5",
			"tool.zip.md5sum",
			"tool.checksums.txt",
			"tool.checksum.txt",
			"TOOL.ZIP.SHA256",
		} {
			assert.Eq(t, AssetChecksum, ClassifyAsset(name), name)
		}
	})

	t.Run("signature", func(t *testing.T) {
		for _, name := range []string{
			"tool.zip.sig",
			"tool.zip.asc",
			"tool.zip.minisig",
			"tool.zip.pem",
			"tool.zip.crt",
			"tool.zip.cert",
			"tool.zip.gpg",
			"tool.zip.sigstore",
			"tool.zip.sigstore.json",
			"TOOL.ZIP.SIG",
		} {
			assert.Eq(t, AssetSignature, ClassifyAsset(name), name)
		}
	})

	t.Run("base name only", func(t *testing.T) {
		assert.Eq(t, AssetChecksum, ClassifyAsset("releases/download/v1.0/tool.zip.sha256"))
	})
}
