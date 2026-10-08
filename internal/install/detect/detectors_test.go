package detect

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestReleaseAssetMetadataClassification(t *testing.T) {
	tests := []struct {
		name     string
		asset    string
		metadata bool
	}{
		{name: "sha256", asset: "tool.exe.sha256", metadata: true},
		{name: "sha256sum", asset: "tool.exe.sha256sum", metadata: true},
		{name: "sha512", asset: "tool.tar.gz.sha512", metadata: true},
		{name: "sha512sum", asset: "tool.tar.gz.sha512sum", metadata: true},
		{name: "md5", asset: "tool.zip.md5", metadata: true},
		{name: "signature", asset: "tool.exe.sig", metadata: true},
		{name: "ascii signature", asset: "tool.tar.gz.asc", metadata: true},
		{name: "minisig", asset: "tool.tar.gz.minisig", metadata: true},
		{name: "sigstore json", asset: "tool-x86_64.zip.sigstore.json", metadata: true},
		{name: "sigstore", asset: "tool-x86_64.zip.sigstore", metadata: true},
		{name: "sbom", asset: "tool.zip.sbom.json", metadata: true},
		{name: "spdx", asset: "tool.spdx.json", metadata: true},
		{name: "cyclonedx", asset: "tool.cyclonedx.json", metadata: true},
		{name: "attestation", asset: "tool.intoto.jsonl", metadata: true},
		{name: "blockmap", asset: "tool.exe.blockmap", metadata: true},
		{name: "latest yml", asset: "latest.yml", metadata: true},
		{name: "latest mac yml", asset: "latest-mac.yml", metadata: true},
		{name: "latest json", asset: "latest.json", metadata: true},
		{name: "releases manifest", asset: "RELEASES", metadata: true},
		{name: "installer", asset: "tool.exe"},
		{name: "archive", asset: "tool.tar.gz"},
		{name: "signal app", asset: "signal-desktop-win.exe"},
		{name: "generic json package", asset: "tool-schema.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Eq(t, tt.metadata, isReleaseMetadataAsset("https://example.com/"+tt.asset))
		})
	}
}

func TestSystemDetectorSkipsReleaseMetadataAssets(t *testing.T) {
	d, err := NewSystemDetector("windows", "amd64")
	assert.NoErr(t, err)

	got, candidates, err := d.Detect([]string{
		"https://example.com/OpenHuman_0.53.43_x64-setup.exe",
		"https://example.com/OpenHuman_0.53.43_x64-setup.exe.sig",
		"https://example.com/OpenHuman_0.53.43_x64-setup.exe.sha512",
		"https://example.com/OpenHuman_0.53.43_x64-setup.exe.blockmap",
		"https://example.com/latest.yml",
	})

	assert.NoErr(t, err)
	assert.Eq(t, "https://example.com/OpenHuman_0.53.43_x64-setup.exe", got)
	assert.Empty(t, candidates)
}

func TestWindowsDetectorSelectsInstallerByExtensionAndArch(t *testing.T) {
	d, err := NewSystemDetector("windows", "amd64")
	assert.NoErr(t, err)

	got, candidates, err := d.Detect([]string{
		"https://example.com/latest.json",
		"https://example.com/openhuman-core-0.54.0-aarch64-unknown-linux-gnu.tar.gz",
		"https://example.com/openhuman-core-0.54.0-x86_64-unknown-linux-gnu.tar.gz",
		"https://example.com/OpenHuman_0.54.0_aarch64-apple-darwin.app.tar.gz",
		"https://example.com/OpenHuman_0.54.0_aarch64.dmg",
		"https://example.com/OpenHuman_0.54.0_amd64.AppImage",
		"https://example.com/OpenHuman_0.54.0_amd64.deb",
		"https://example.com/OpenHuman_0.54.0_x64-setup.exe",
		"https://example.com/OpenHuman_0.54.0_x64.dmg",
		"https://example.com/OpenHuman_0.54.0_x64_en-US.msi",
		"https://example.com/OpenHuman_0.54.0_x86_64-apple-darwin.app.tar.gz",
	})

	assert.NoErr(t, err)
	assert.Eq(t, "https://example.com/OpenHuman_0.54.0_x64-setup.exe", got)
	assert.Empty(t, candidates)
}

func TestWindowsDetectorShowsArchivesWhenMultipleExecutablesMatch(t *testing.T) {
	d, err := NewSystemDetector("windows", "amd64")
	assert.NoErr(t, err)

	got, candidates, err := d.Detect([]string{
		"https://example.com/codex-app-server-x86_64-pc-windows-msvc.exe",
		"https://example.com/codex-app-server-x86_64-pc-windows-msvc.zip",
		"https://example.com/codex-command-runner-x86_64-pc-windows-msvc.exe",
		"https://example.com/codex-command-runner-x86_64-pc-windows-msvc.tar.gz",
		"https://example.com/codex-x86_64-unknown-linux-gnu.tar.gz",
	})

	assert.Err(t, err)
	assert.Eq(t, "", got)
	assert.Eq(t, []string{
		"https://example.com/codex-app-server-x86_64-pc-windows-msvc.exe",
		"https://example.com/codex-app-server-x86_64-pc-windows-msvc.zip",
		"https://example.com/codex-command-runner-x86_64-pc-windows-msvc.exe",
		"https://example.com/codex-command-runner-x86_64-pc-windows-msvc.tar.gz",
	}, candidates)
}

func TestLinuxDetectorPrefersGlibcAsset(t *testing.T) {
	d, err := NewSystemDetectorWithLibc("linux", "amd64", "glibc")
	assert.NoErr(t, err)

	got, candidates, err := d.Detect([]string{
		"https://example.com/starship-x86_64-unknown-linux-gnu.tar.gz",
		"https://example.com/starship-x86_64-unknown-linux-musl.tar.gz",
	})

	assert.NoErr(t, err)
	assert.Eq(t, "https://example.com/starship-x86_64-unknown-linux-gnu.tar.gz", got)
	assert.Empty(t, candidates)
}

func TestLinuxDetectorPrefersMuslAsset(t *testing.T) {
	d, err := NewSystemDetectorWithLibc("linux", "amd64", "musl")
	assert.NoErr(t, err)

	got, candidates, err := d.Detect([]string{
		"https://example.com/starship-x86_64-unknown-linux-gnu.tar.gz",
		"https://example.com/starship-x86_64-unknown-linux-musl.tar.gz",
	})

	assert.NoErr(t, err)
	assert.Eq(t, "https://example.com/starship-x86_64-unknown-linux-musl.tar.gz", got)
	assert.Empty(t, candidates)
}

func TestAssetDetectorSkipsReleaseMetadataAssets(t *testing.T) {
	d := &assetDetector{Asset: "exe"}

	got, candidates, err := d.Detect([]string{
		"https://example.com/OpenHuman_0.53.43_x64-setup.exe",
		"https://example.com/OpenHuman_0.53.43_x64-setup.exe.sig",
	})

	assert.NoErr(t, err)
	assert.Eq(t, "https://example.com/OpenHuman_0.53.43_x64-setup.exe", got)
	assert.Empty(t, candidates)
}

func TestAllDetectorSkipsReleaseMetadataAssets(t *testing.T) {
	d := &allDetector{}

	got, candidates, err := d.Detect([]string{
		"https://example.com/OpenHuman_0.53.43_x64-setup.exe",
		"https://example.com/OpenHuman_0.53.43_x64-setup.exe.sig",
	})

	assert.NoErr(t, err)
	assert.Eq(t, "https://example.com/OpenHuman_0.53.43_x64-setup.exe", got)
	assert.Empty(t, candidates)
}
