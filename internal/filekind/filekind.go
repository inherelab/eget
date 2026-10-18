// Package filekind holds only the "file name → kind" rule data and query
// primitives: multi-segment extensions, archive/executable/installer suffix
// sets and the release asset classification.
//
// It deliberately holds no decision policy: it does not build extractors, touch
// the file system, or decide platform/match semantics. Every exported set is
// named after its consuming site and keeps that site's original suffix list and
// case axis unchanged — migration equivalence is a hard requirement, see
// docs/plans/2026-10-17-rule-consolidation-plan.md.
package filekind

// Kind is the intermediate "file name → archive/compression kind" result. The
// caller maps it to a concrete extractor; the mapping stays in package install
// because the constructors live there.
type Kind int

// Archive/compression kinds produced by kindSuffixes.
const (
	KindUnknown Kind = iota
	KindTarGz
	KindTarBz2
	KindTarXz
	KindTarZst
	KindTar
	KindZip
	KindSevenZip
	KindGz
	KindBz2
	KindXz
	KindZst
)

// Single suffixes that several stations compare against after taking them from
// a set; exporting them keeps the literal in one place.
const (
	ExtExe      = ".exe"
	ExtAppImage = ".appimage"
	ExtMsi      = ".msi"
)

// AssetClass is the release asset classification result.
type AssetClass int

// Release asset classes.
const (
	AssetInstallable AssetClass = iota
	AssetChecksum
	AssetSignature
	AssetMetadata
)

// Named suffix sets. Each set mirrors the sites that consume it: never merge two
// sets that differ in content, because the sites differ in intent
// (ExtSetExtractor carries the single-file compressors, ExtSetArchiveMulti only
// the multi-segment archives).
var (
	// ExtSetExtractor is the suffix list of install.NewExtractor.
	ExtSetExtractor = []string{
		".tar.gz", ".tgz",
		".tar.bz2", ".tbz",
		".tar.xz", ".txz",
		".tar.zst",
		".tar",
		".zip",
		".7z",
		".gz",
		".bz2",
		".xz",
		".zst",
	}

	// ExtSetExtractAll is the suffix list of install.isExtractAllArchiveAsset
	// (archives only, no single-file compressors).
	ExtSetExtractAll = []string{
		".zip", ".7z", ".tar",
		".tar.gz", ".tgz",
		".tar.bz2", ".tbz",
		".tar.xz", ".txz",
		".tar.zst",
	}

	// ExtSetArchiveMulti is the multi-segment archive list shared by
	// client.archiveExt, app/cache.cacheAssetExt and the sdk index parsers.
	ExtSetArchiveMulti = []string{
		".tar.gz", ".tar.xz", ".tar.bz2", ".tar.zst", ".tar.br", ".tar.lz4",
	}

	// ExtSetDownload is the suffix list of install.isInvalidCachedDownload.
	ExtSetDownload = []string{".zip", ".gz", ".tgz", ".xz", ".bz2", ".zst", ".7z", ".rar"}

	// ExtSetSystemArchiver is the unconditional set of
	// install.shouldUseSystem7z; the ".exe" case is conditional and stays at the
	// call site as IsExecutableFold(name) && extractArchive.
	ExtSetSystemArchiver = []string{".7z", ".rar", ".msi", ".cab", ".iso"}

	// ExtSetNonExecutable is the suffix list of install.isDefinitelyNotExec.
	// The base-name list (license/readme/...) stays in that function.
	ExtSetNonExecutable = []string{".deb", ".1", ".txt"}

	// ExtSetExecutable is the executable suffix list used with either case axis.
	ExtSetExecutable = []string{ExtExe, ExtAppImage}

	// ExtSetInstaller is the installer suffix list of install.DetectGUIInstallMode
	// and DetectInstallerKind.
	ExtSetInstaller = []string{ExtExe, ExtMsi}
)

// kindSuffixes maps the ExtSetExtractor suffixes to Kind values. Order matters:
// multi-segment suffixes must be probed before their single-segment tails
// (".tar.gz" before ".gz").
var kindSuffixes = []struct {
	ext  string
	kind Kind
}{
	{".tar.gz", KindTarGz},
	{".tgz", KindTarGz},
	{".tar.bz2", KindTarBz2},
	{".tbz", KindTarBz2},
	{".tar.xz", KindTarXz},
	{".txz", KindTarXz},
	{".tar.zst", KindTarZst},
	{".tar", KindTar},
	{".zip", KindZip},
	{".7z", KindSevenZip},
	{".gz", KindGz},
	{".bz2", KindBz2},
	{".xz", KindXz},
	{".zst", KindZst},
}
