package filekind

import (
	"path"
	"strings"
)

// metadataNames are exact asset names that carry release metadata.
var metadataNames = map[string]bool{
	"releases":         true,
	"latest.yml":       true,
	"latest-mac.yml":   true,
	"latest-linux.yml": true,
	"latest.json":      true,
}

var metadataSuffixes = []string{
	".blockmap", ".yml", ".yaml", ".sbom.json", ".spdx.json", ".cyclonedx.json",
	".intoto", ".intoto.jsonl", ".provenance", ".attestation",
}

var checksumSuffixes = []string{
	".sha256", ".sha256sum", ".sha512", ".sha512sum", ".md5", ".md5sum",
	".checksums.txt", ".checksum.txt",
}

var signatureSuffixes = []string{
	".sig", ".asc", ".minisig", ".pem", ".crt", ".cert", ".gpg", ".sigstore", ".sigstore.json",
}

// ClassifyAsset classifies a release asset by base name, lower-cased first, in
// the original order of install/detect.classifyReleaseAsset: metadata, checksum,
// signature, then installable.
func ClassifyAsset(asset string) AssetClass {
	name := strings.ToLower(path.Base(asset))
	switch {
	case metadataNames[name] || hasSuffixFold(name, metadataSuffixes):
		return AssetMetadata
	case hasSuffixFold(name, checksumSuffixes):
		return AssetChecksum
	case hasSuffixFold(name, signatureSuffixes):
		return AssetSignature
	default:
		return AssetInstallable
	}
}
