package install

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// This file pins down the file-name → kind behaviour of every suffix station
// listed in docs/plans/2026-10-17-rule-consolidation-plan.md before those
// stations start consuming internal/filekind. The assertions must keep passing
// unchanged after the migration: they are the equivalence guard.

// decompressSample is the plain text the codec payloads below decode to.
const decompressSample = "hello eget\n"

// bz2Sample is `python -c "import bz2;print(bz2.compress(b'hello eget\n',9))"`;
// Go's stdlib has a bzip2 reader but no writer, so the payload is embedded.
var bz2Sample = []byte{
	0x42, 0x5a, 0x68, 0x39, 0x31, 0x41, 0x59, 0x26, 0x53, 0x59, 0xbe, 0x66, 0x7b, 0x5d,
	0x00, 0x00, 0x03, 0x51, 0x80, 0x00, 0x10, 0x40, 0x00, 0x02, 0xc4, 0x84, 0x00, 0x20,
	0x00, 0x31, 0x0c, 0x08, 0x21, 0xa1, 0xb4, 0x92, 0x68, 0x02, 0x94, 0xbc, 0x5d, 0xc9,
	0x14, 0xe1, 0x42, 0x42, 0xf9, 0x99, 0xed, 0x74,
}

func gzSample(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write([]byte(decompressSample))
	assert.NoErr(t, err)
	assert.NoErr(t, w.Close())
	return buf.Bytes()
}

func xzSample(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := xz.NewWriter(&buf)
	assert.NoErr(t, err)
	_, err = w.Write([]byte(decompressSample))
	assert.NoErr(t, err)
	assert.NoErr(t, w.Close())
	return buf.Bytes()
}

func zstdSample(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf)
	assert.NoErr(t, err)
	_, err = w.Write([]byte(decompressSample))
	assert.NoErr(t, err)
	assert.NoErr(t, w.Close())
	return buf.Bytes()
}

func fnPointer(fn any) uintptr {
	return reflect.ValueOf(fn).Pointer()
}

// TestEquivS_A01 pins NewExtractor: which archive constructor and which
// decompressor each suffix selects.
func TestEquivS_A01(t *testing.T) {
	type kindCase struct {
		name          string
		archive       ArchiveFn
		payload       []byte
		nilDecompress bool
	}
	cases := []kindCase{
		{name: "tool_1.0_linux_amd64.tar.gz", archive: NewTarArchive, payload: gzSample(t)},
		{name: "tool_1.0_linux_amd64.tgz", archive: NewTarArchive, payload: gzSample(t)},
		{name: "tool_1.0_linux_amd64.tar.bz2", archive: NewTarArchive, payload: bz2Sample},
		{name: "tool_1.0_linux_amd64.tbz", archive: NewTarArchive, payload: bz2Sample},
		{name: "tool_1.0_linux_amd64.tar.xz", archive: NewTarArchive, payload: xzSample(t)},
		{name: "tool_1.0_linux_amd64.txz", archive: NewTarArchive, payload: xzSample(t)},
		{name: "tool_1.0_linux_amd64.tar.zst", archive: NewTarArchive, payload: zstdSample(t)},
		{name: "tool_1.0_linux_amd64.tar", archive: NewTarArchive},
		{name: "tool_1.0_linux_amd64.zip", archive: NewZipArchive, nilDecompress: true},
		{name: "tool_1.0_linux_amd64.7z", archive: NewSevenZipArchive, nilDecompress: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ext := NewExtractor(tc.name, "tool", nil)
			ae, ok := ext.(*ArchiveExtractor)
			assert.True(t, ok, tc.name)
			assert.Eq(t, fnPointer(tc.archive), fnPointer(ae.Ar), tc.name)

			if tc.nilDecompress {
				assert.Nil(t, ae.Decompress, tc.name)
				return
			}
			if tc.payload == nil {
				// No decompression step: the reader is handed through as-is.
				src := bytes.NewReader([]byte(decompressSample))
				got, err := ae.Decompress(src)
				assert.NoErr(t, err)
				assert.Eq(t, decompressSample, readAll(t, got))
				return
			}
			got, err := ae.Decompress(bytes.NewReader(tc.payload))
			assert.NoErr(t, err)
			assert.Eq(t, decompressSample, readAll(t, got))
		})
	}

	t.Run("single file compressors", func(t *testing.T) {
		single := []struct {
			name    string
			payload []byte
		}{
			{"tool_1.0_linux_amd64.gz", gzSample(t)},
			{"tool_1.0_linux_amd64.bz2", bz2Sample},
			{"tool_1.0_linux_amd64.xz", xzSample(t)},
			{"tool_1.0_linux_amd64.zst", zstdSample(t)},
		}
		for _, tc := range single {
			se, ok := NewExtractor(tc.name, "tool", nil).(*SingleFileExtractor)
			assert.True(t, ok, tc.name)
			assert.Eq(t, tc.name, se.Name, tc.name)
			got, err := se.Decompress(bytes.NewReader(tc.payload))
			assert.NoErr(t, err)
			assert.Eq(t, decompressSample, readAll(t, got), tc.name)
		}
	})

	t.Run("unknown suffix stays a plain single file", func(t *testing.T) {
		se, ok := NewExtractor("tool.exe", "tool", nil).(*SingleFileExtractor)
		assert.True(t, ok)
		src := bytes.NewReader([]byte(decompressSample))
		got, err := se.Decompress(src)
		assert.NoErr(t, err)
		assert.Eq(t, decompressSample, readAll(t, got))
	})

	t.Run("case sensitive axis", func(t *testing.T) {
		// Upper case suffixes are not recognised: the name is used verbatim.
		se, ok := NewExtractor("TOOL_1.0.TAR.GZ", "tool", nil).(*SingleFileExtractor)
		assert.True(t, ok)
		assert.Eq(t, "TOOL_1.0.TAR.GZ", se.Name)
	})
}

func readAll(t *testing.T, r io.Reader) string {
	t.Helper()
	data, err := io.ReadAll(r)
	assert.NoErr(t, err)
	return string(data)
}

// TestEquivS_A02 pins isExtractAllArchiveAsset (folded axis).
func TestEquivS_A02(t *testing.T) {
	for _, name := range []string{
		"https://x/tool_1.0_linux_amd64.zip",
		"https://x/tool.7z",
		"https://x/tool.tar",
		"https://x/tool.tar.gz",
		"https://x/tool.tgz",
		"https://x/tool.tar.bz2",
		"https://x/tool.tbz",
		"https://x/tool.tar.xz",
		"https://x/tool.txz",
		"https://x/tool.tar.zst",
		"https://x/TOOL.TAR.ZST",
	} {
		assert.True(t, isExtractAllArchiveAsset(name), name)
	}
	for _, name := range []string{"https://x/tool.gz", "https://x/tool.tar.br", "https://x/tool.exe"} {
		assert.False(t, isExtractAllArchiveAsset(name), name)
	}
}

// TestEquivS_A03 pins isInvalidCachedDownload: the suffix gate (folded) plus the
// HTML error page probe.
func TestEquivS_A03(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		assert.NoErr(t, os.WriteFile(p, []byte(content), 0o644))
		return p
	}

	html := write("tool.zip", "<!DOCTYPE html><html><body>404</body></html>")
	assert.True(t, isInvalidCachedDownload(html))
	assert.True(t, isInvalidCachedDownload(write("TOOL.ZIP", "<HTML>nope</HTML>")))
	assert.True(t, isInvalidCachedDownload(write("tool.tar.gz", "<html>nope</html>")))

	assert.False(t, isInvalidCachedDownload(write("tool.exe", "<html>nope</html>")))
	assert.False(t, isInvalidCachedDownload(write("tool.tar.br", "<html>nope</html>")))
	assert.False(t, isInvalidCachedDownload(write("tool0.bin", "<html>nope</html>")))
	assert.False(t, isInvalidCachedDownload(filepath.Join(dir, "missing.zip")))
}

// TestEquivS_E01 pins install.rename (case sensitive axis).
func TestEquivS_E01(t *testing.T) {
	assert.Eq(t, "tool", rename("tool.appimage", "guess"))
	assert.Eq(t, "tool.exe", rename("tool.exe", "guess"))
	assert.Eq(t, "guess", rename("tool.zip", "guess"))
	assert.Eq(t, "README.txt", rename("README.txt", "guess"))
	assert.Eq(t, "LICENSE", rename("LICENSE", "guess"))
	// Upper case suffixes are not stripped by the sensitive axis.
	assert.Eq(t, "guess", rename("TOOL.APPIMAGE", "guess"))
	assert.Eq(t, "guess", rename("TOOL.EXE", "guess"))
}

// TestEquivS_E02 pins isDefinitelyNotExec / isExec (mixed axis).
func TestEquivS_E02(t *testing.T) {
	t.Run("isDefinitelyNotExec", func(t *testing.T) {
		assert.True(t, isDefinitelyNotExec("pkg.deb"))
		assert.True(t, isDefinitelyNotExec("man.1"))
		assert.True(t, isDefinitelyNotExec("notes.txt"))
		assert.True(t, isDefinitelyNotExec("README"))
		assert.True(t, isDefinitelyNotExec("dir/LICENSE"))
		assert.False(t, isDefinitelyNotExec("PKG.DEB"))
		assert.False(t, isDefinitelyNotExec("NOTES.TXT"))
		assert.False(t, isDefinitelyNotExec("tool.exe"))
	})

	t.Run("isExec", func(t *testing.T) {
		assert.True(t, isExec("tool.exe", 0))
		assert.True(t, isExec("tool.appimage", 0))
		assert.True(t, isExec("tool", 0o644))
		assert.True(t, isExec("tool.bin", 0o755))
		assert.False(t, isExec("TOOL.EXE", 0))
		assert.False(t, isExec("tool.bin", 0o644))
		assert.False(t, isExec("notes.txt", 0o755))
	})
}

// TestEquivS_E03 pins BinaryChooser.Choose (case sensitive axis).
func TestEquivS_E03(t *testing.T) {
	chooser := &BinaryChooser{Tool: "tool"}
	ok, possible := chooser.Choose("tool", false, 0o755)
	assert.True(t, ok)
	assert.True(t, possible)

	ok, possible = chooser.Choose("dir/tool.exe", false, 0o644)
	assert.True(t, ok)
	assert.True(t, possible)

	ok, possible = chooser.Choose("TOOL.EXE", false, 0o755)
	assert.False(t, ok)
	assert.True(t, possible)

	ok, possible = chooser.Choose("tool.exe", true, 0o755)
	assert.False(t, ok)
	assert.False(t, possible)
}

// TestEquivS_E04 pins isExecutableForGOOS (folded axis).
func TestEquivS_E04(t *testing.T) {
	assert.True(t, isExecutableForGOOS(ExtractedFile{Name: "tool.exe"}, "windows"))
	assert.True(t, isExecutableForGOOS(ExtractedFile{Name: "TOOL.EXE"}, "windows"))
	assert.False(t, isExecutableForGOOS(ExtractedFile{Name: "tool"}, "windows"))
	assert.False(t, isExecutableForGOOS(ExtractedFile{Name: "tool", Dir: true}, "windows"))

	assert.True(t, isExecutableForGOOS(ExtractedFile{Name: "tool"}, "linux"))
	assert.True(t, isExecutableForGOOS(ExtractedFile{Name: "tool.bin", mode: 0o755}, "linux"))
	assert.False(t, isExecutableForGOOS(ExtractedFile{Name: "tool.exe"}, "linux"))
	assert.False(t, isExecutableForGOOS(ExtractedFile{Name: "tool.bin", mode: 0o644}, "linux"))
}

// TestEquivS_E05 pins autoSelectOnlyWindowsExecutable (folded axis).
func TestEquivS_E05(t *testing.T) {
	selected, ok := autoSelectOnlyWindowsExecutable([]ExtractedFile{{ArchiveName: "tool.exe"}})
	assert.True(t, ok)
	assert.Eq(t, "tool.exe", selected.ArchiveName)

	_, ok = autoSelectOnlyWindowsExecutable([]ExtractedFile{{ArchiveName: "TOOL.EXE"}})
	assert.True(t, ok)

	_, ok = autoSelectOnlyWindowsExecutable([]ExtractedFile{{ArchiveName: "tool.exe"}, {ArchiveName: "other.exe"}})
	assert.False(t, ok)

	_, ok = autoSelectOnlyWindowsExecutable([]ExtractedFile{{ArchiveName: "tool.tar.gz"}})
	assert.False(t, ok)
}

// TestEquivS_E06 pins applyPreferredName / shouldUsePreferredExecutableName.
func TestEquivS_E06(t *testing.T) {
	assert.Eq(t, "tool_1.0_linux_amd64.exe", applyPreferredName("tool_1.0_linux_amd64.exe", ""))
	// preferredName already carries an extension (.0), so it wins as-is.
	assert.Eq(t, "tool_1.0", applyPreferredName("tool_1.0_linux_amd64.exe", "tool_1.0"))
	assert.Eq(t, "tool.exe", applyPreferredName("TOOL_1.0_LINUX_AMD64.EXE", "tool"))
	assert.Eq(t, "renamed.tar.gz", applyPreferredName("tool_1.0_linux_amd64.exe", "renamed.tar.gz"))

	assert.True(t, shouldUsePreferredExecutableName("tool_1.0_linux_amd64.exe", "tool"))
	assert.True(t, shouldUsePreferredExecutableName("tool_1.0_linux_amd64.exe", "tool_1.0_linux_amd64.exe"))
	assert.False(t, shouldUsePreferredExecutableName("tool_1.0_linux_amd64.exe", "other"))
	assert.False(t, shouldUsePreferredExecutableName("tool_1.0_linux_amd64.exe", ""))
}

// TestEquivS_E07 pins heuristicExecutableName / executableSuffix (folded axis).
func TestEquivS_E07(t *testing.T) {
	assert.Eq(t, ".exe", executableSuffix("tool_1.0_linux_amd64.exe"))
	assert.Eq(t, ".exe", executableSuffix("TOOL_1.0.EXE"))
	assert.Eq(t, ".appimage", executableSuffix("TOOL.APPIMAGE"))
	assert.Eq(t, "", executableSuffix("tool.zip"))

	assert.Eq(t, "tool_1.0.exe", heuristicExecutableName("tool_1.0_linux_amd64.exe"))
	// Mixed axis quirk: the suffix probe folds case but the trim does not, so an
	// upper-case suffix leaves the name untouched.
	assert.Eq(t, "TOOL_1.0_LINUX_AMD64.EXE", heuristicExecutableName("TOOL_1.0_LINUX_AMD64.EXE"))
	assert.Eq(t, "tool_1.0.exe", heuristicExecutableName("tool_1.0_amd64_linux.exe"))
	assert.Eq(t, "tool.exe", heuristicExecutableName("tool-linux.exe"))
	assert.Eq(t, "tool_1.0.exe", heuristicExecutableName("tool_1.0_linux_armv7.exe"))
	assert.Eq(t, "tool.appimage", heuristicExecutableName("tool.appimage"))
}

// TestEquivS_E08 pins normalizedAssetNameHint (folded axis).
func TestEquivS_E08(t *testing.T) {
	assert.Eq(t, "tool", normalizedAssetNameHint("TOOL.EXE"))
	assert.Eq(t, "tool", normalizedAssetNameHint("  Tool.Exe "))
	assert.Eq(t, "tool", normalizedAssetNameHint("tool.appimage"))
	assert.Eq(t, "tool.tar.gz", normalizedAssetNameHint("tool.tar.gz"))
	assert.Eq(t, "", normalizedAssetNameHint("  "))
}

// TestEquivS_E09 pins DetectGUIInstallMode (folded axis).
func TestEquivS_E09(t *testing.T) {
	assert.Eq(t, "", DetectGUIInstallMode(false, "tool.exe"))
	assert.Eq(t, InstallModeInstaller, DetectGUIInstallMode(true, "tool.exe"))
	assert.Eq(t, InstallModeInstaller, DetectGUIInstallMode(true, "dir/TOOL.EXE"))
	assert.Eq(t, InstallModeInstaller, DetectGUIInstallMode(true, "tool.msi"))
	assert.Eq(t, InstallModePortable, DetectGUIInstallMode(true, "tool.zip"))
	assert.Eq(t, InstallModePortable, DetectGUIInstallMode(true, "tool-portable.exe"))
}

// TestEquivS_E10 pins DetectInstallerKind (folded axis).
func TestEquivS_E10(t *testing.T) {
	assert.Eq(t, InstallerKindMSI, DetectInstallerKind("tool.msi"))
	assert.Eq(t, InstallerKindEXE, DetectInstallerKind("tool-setup.exe"))
	assert.Eq(t, InstallerKindEXE, DetectInstallerKind("TOOL-INSTALLER.EXE"))
	assert.Eq(t, InstallerKindUnknown, DetectInstallerKind("tool.exe"))
	assert.Eq(t, InstallerKindUnknown, DetectInstallerKind("tool.appimage"))
}

// TestEquivS_E11 pins requiresSystem7z: only ".exe" plus extractArchive.
func TestEquivS_E11(t *testing.T) {
	assert.True(t, requiresSystem7z("tool.exe", true))
	assert.True(t, requiresSystem7z("TOOL.EXE", true))
	assert.False(t, requiresSystem7z("tool.exe", false))
	for _, name := range []string{"tool.7z", "tool.rar", "tool.msi", "tool.cab", "tool.iso", "tool.tar.gz"} {
		assert.False(t, requiresSystem7z(name, true), name)
	}
}

// TestEquivS_E12 pins shouldUseSystem7z: unconditional suffixes plus the
// conditional ".exe" arm.
func TestEquivS_E12(t *testing.T) {
	for _, name := range []string{"tool.7z", "tool.rar", "tool.msi", "tool.cab", "tool.iso"} {
		assert.True(t, shouldUseSystem7z(name, false), name)
		assert.True(t, shouldUseSystem7z(strings.ToUpper(name), false), name)
	}
	assert.True(t, shouldUseSystem7z("tool.exe", true))
	assert.False(t, shouldUseSystem7z("tool.exe", false))
	assert.False(t, shouldUseSystem7z("tool.tar.gz", true))
}
