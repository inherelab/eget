package install

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bodgit/sevenzip"
	"github.com/gobwas/glob"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"

	forge "github.com/inherelab/eget/internal/source/forge"
	sourcegithub "github.com/inherelab/eget/internal/source/github"
	sourcesf "github.com/inherelab/eget/internal/source/sourceforge"
)

type Extractor interface {
	Extract(data []byte, multiple bool) (ExtractedFile, []ExtractedFile, error)
}

type ExtractedFile struct {
	Name        string
	ArchiveName string
	mode        fs.FileMode
	Extract     func(to string) error
	Dir         bool
}

func (e ExtractedFile) Mode() fs.FileMode {
	return modeFrom(e.Name, e.mode)
}

func (e ExtractedFile) String() string {
	return e.ArchiveName
}

type Chooser interface {
	Choose(name string, dir bool, mode fs.FileMode) (direct bool, possible bool)
}

type FileType byte

const (
	TypeNormal FileType = iota
	TypeDir
	TypeLink
	TypeSymlink
	TypeOther
)

type File struct {
	Name     string
	LinkName string
	Mode     fs.FileMode
	Type     FileType
}

func (f File) Dir() bool {
	return f.Type == TypeDir
}

type Archive interface {
	Next() (File, error)
	ReadAll() ([]byte, error)
}

type ArchiveFn func(data []byte, decomp DecompFn) (Archive, error)
type DecompFn func(r io.Reader) (io.Reader, error)

type TarArchive struct {
	r *tar.Reader
}

type ZipArchive struct {
	r   *zip.Reader
	idx int
}

type ArchiveExtractor struct {
	File       Chooser
	Ar         ArchiveFn
	Decompress DecompFn
}

type SingleFileExtractor struct {
	Rename     string
	Name       string
	Decompress func(r io.Reader) (io.Reader, error)
}

type BinaryChooser struct {
	Tool string
}

type LiteralFileChooser struct {
	File string
}

type GlobChooser struct {
	expr string
	g    glob.Glob
	all  bool
}

type MultiChooser struct {
	expr     string
	choosers []Chooser
}

type noVerifier struct{}

type sha256Error struct {
	Expected []byte
	Got      []byte
}

type sha256Verifier struct {
	Expected []byte
}

type sha256Printer struct{}

type sha256AssetVerifier struct {
	AssetURL string
	Getter   sourcegithub.HTTPGetter
}

type detectorChain struct {
	detectors []Detector
	system    Detector
}

type assetDetector struct {
	Asset string
	Anti  bool
	Regex *regexp.Regexp
}

type allDetector struct{}

type systemOS struct {
	name     string
	regex    *regexp.Regexp
	anti     *regexp.Regexp
	priority *regexp.Regexp
}

type systemArch struct {
	name  string
	regex *regexp.Regexp
}

type systemDetector struct {
	Os   systemOS
	Arch systemArch
}

func NewDefaultService(githubGetter sourcegithub.HTTPGetter, binaryModTime func(tool, output string) time.Time) *Service {
	return &Service{
		BinaryModTime: binaryModTime,
		GitHubGetter:  githubGetter,
		GitHubGetterFactory: func(opts Options) sourcegithub.HTTPGetter {
			return NewHTTPGetter(opts)
		},
		ForgeGetterFactory: func(opts Options) forge.HTTPGetter {
			return NewHTTPGetter(opts)
		},
		SourceForgeGetterFactory: func(opts Options) sourcesf.HTTPGetter {
			return NewHTTPGetter(opts)
		},
		AllDetectorFactory: func() Detector {
			return &allDetector{}
		},
		SystemDetectorFactory: func(goos, goarch string) (Detector, error) {
			return newSystemDetector(goos, goarch)
		},
		AssetDetectorFactory: func(asset string, anti bool, re *regexp.Regexp) Detector {
			return &assetDetector{Asset: asset, Anti: anti, Regex: re}
		},
		DetectorChainFactory: func(detectors []Detector, system Detector) Detector {
			return &detectorChain{detectors: detectors, system: system}
		},
		Sha256VerifierFactory: func(expected string) (Verifier, error) {
			return newSha256Verifier(expected)
		},
		Sha256AssetVerifierFactory: func(assetURL string, opts Options) Verifier {
			getter := githubGetter
			if getter == nil {
				getter = NewHTTPGetter(opts)
			}
			return &sha256AssetVerifier{AssetURL: assetURL, Getter: getter}
		},
		Sha256PrinterFactory: func() Verifier {
			return &sha256Printer{}
		},
		NoVerifierFactory: func() Verifier {
			return &noVerifier{}
		},
		DownloadOnlyExtractorFactory: func(name string) any {
			return NewDownloadOnlyExtractor(name)
		},
		GlobChooserFactory: func(pattern string) (any, error) {
			return NewFileChooser(pattern)
		},
		BinaryChooserFactory: func(tool string) any {
			return NewBinaryChooser(tool)
		},
		ExtractorFactory: func(filename, tool string, chooser any) any {
			return NewExtractor(filename, tool, chooser.(Chooser))
		},
	}
}

func (dc *detectorChain) Detect(assets []string) (string, []string, error) {
	for _, d := range dc.detectors {
		choice, candidates, err := d.Detect(assets)
		if len(candidates) == 0 && err != nil {
			return "", nil, err
		}
		if len(candidates) == 0 {
			return choice, nil, nil
		}
		assets = candidates
	}
	choice, candidates, err := dc.system.Detect(assets)
	if len(candidates) == 0 && err != nil {
		return "", nil, err
	}
	if len(candidates) == 0 {
		return choice, nil, nil
	}
	return "", candidates, fmt.Errorf("%d candidates found for asset chain", len(candidates))
}

func (a *allDetector) Detect(assets []string) (string, []string, error) {
	if len(assets) == 1 {
		return assets[0], nil, nil
	}
	return "", assets, fmt.Errorf("%d matches found", len(assets))
}

func (s *assetDetector) Detect(assets []string) (string, []string, error) {
	var candidates []string
	for _, a := range assets {
		base := path.Base(a)
		if !s.Anti && base == s.Asset {
			return a, nil, nil
		}
		if !s.Anti {
			if s.matches(base) {
				candidates = append(candidates, a)
			}
		}
		if s.Anti && !s.matches(base) {
			candidates = append(candidates, a)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil, nil
	}
	if len(candidates) > 1 {
		return "", candidates, fmt.Errorf("%d candidates found for asset `%s`", len(candidates), s.Asset)
	}
	return "", nil, fmt.Errorf("asset `%s` not found", s.Asset)
}

func (s *assetDetector) matches(base string) bool {
	if s.Regex != nil {
		return s.Regex.MatchString(base)
	}
	return strings.Contains(strings.ToLower(base), strings.ToLower(s.Asset))
}

func compileAssetRegex(expr string) (*regexp.Regexp, error) {
	return regexp.Compile(expr)
}

func (osv *systemOS) Match(s string) (bool, bool) {
	if osv.anti != nil && osv.anti.MatchString(s) {
		return false, false
	}
	if osv.priority != nil {
		return osv.regex.MatchString(s), osv.priority.MatchString(s)
	}
	return osv.regex.MatchString(s), false
}

func (a *systemArch) Match(s string) bool {
	return a.regex.MatchString(s)
}

func newSystemDetector(goos, goarch string) (*systemDetector, error) {
	osv, ok := installGOOSMap[goos]
	if !ok {
		return nil, fmt.Errorf("unsupported target OS: %s", goos)
	}
	arch, ok := installGOARCHMap[goarch]
	if !ok {
		return nil, fmt.Errorf("unsupported target arch: %s", goarch)
	}
	return &systemDetector{Os: osv, Arch: arch}, nil
}

func (d *systemDetector) Detect(assets []string) (string, []string, error) {
	var priority []string
	var matches []string
	var candidates []string
	all := make([]string, 0, len(assets))
	for _, a := range assets {
		if strings.HasSuffix(a, ".sha256") || strings.HasSuffix(a, ".sha256sum") {
			continue
		}
		osMatch, extra := d.Os.Match(a)
		if extra {
			priority = append(priority, a)
		}
		archMatch := d.Arch.Match(a)
		if osMatch && archMatch {
			matches = append(matches, a)
		}
		if osMatch {
			candidates = append(candidates, a)
		}
		all = append(all, a)
	}
	if len(priority) == 1 {
		return priority[0], nil, nil
	}
	if len(priority) > 1 {
		return "", priority, fmt.Errorf("%d priority matches found", len(matches))
	}
	if len(matches) == 1 {
		return matches[0], nil, nil
	}
	if len(matches) > 1 {
		return "", matches, fmt.Errorf("%d matches found", len(matches))
	}
	if len(candidates) == 1 {
		return candidates[0], nil, nil
	}
	if len(candidates) > 1 {
		return "", candidates, fmt.Errorf("%d candidates found (unsure architecture)", len(candidates))
	}
	if len(all) == 1 {
		return all[0], nil, nil
	}
	return "", all, fmt.Errorf("no candidates found")
}

func (n *noVerifier) Verify(b []byte) error {
	return nil
}

func (e *sha256Error) Error() string {
	return fmt.Sprintf("sha256 checksum mismatch:\nexpected: %x\ngot:      %x", e.Expected, e.Got)
}

func newSha256Verifier(expectedHex string) (*sha256Verifier, error) {
	expected, _ := hex.DecodeString(expectedHex)
	if len(expected) != sha256.Size {
		return nil, fmt.Errorf("sha256sum (%s) too small: %d bytes decoded", expectedHex, len(expectedHex))
	}
	return &sha256Verifier{Expected: expected}, nil
}

func (s *sha256Verifier) Verify(b []byte) error {
	sum := sha256.Sum256(b)
	if bytes.Equal(sum[:], s.Expected) {
		return nil
	}
	return &sha256Error{Expected: s.Expected, Got: sum[:]}
}

func (s *sha256Printer) Verify(b []byte) error {
	sum := sha256.Sum256(b)
	fmt.Printf("%x\n", sum)
	return nil
}

func (s *sha256AssetVerifier) Verify(b []byte) error {
	if s.Getter == nil {
		return fmt.Errorf("github getter is required")
	}
	resp, err := s.Getter.Get(s.AssetURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	expected := make([]byte, sha256.Size)
	n, err := hex.Decode(expected, data)
	if n < sha256.Size {
		return fmt.Errorf("sha256sum (%s) too small: %d bytes decoded", string(data), n)
	}
	sum := sha256.Sum256(b)
	if bytes.Equal(sum[:], expected[:n]) {
		return nil
	}
	return &sha256Error{Expected: expected[:n], Got: sum[:]}
}

func NewExtractor(filename string, tool string, chooser Chooser) Extractor {
	if tool == "" {
		tool = filename
	}
	gunzipper := func(r io.Reader) (io.Reader, error) { return gzip.NewReader(r) }
	b2unzipper := func(r io.Reader) (io.Reader, error) { return bzip2.NewReader(r), nil }
	xunzipper := func(r io.Reader) (io.Reader, error) { return xz.NewReader(bufio.NewReader(r)) }
	zstdunzipper := func(r io.Reader) (io.Reader, error) { return zstd.NewReader(r) }
	nounzipper := func(r io.Reader) (io.Reader, error) { return r, nil }

	switch {
	case strings.HasSuffix(filename, ".tar.gz"), strings.HasSuffix(filename, ".tgz"):
		return NewArchiveExtractor(chooser, NewTarArchive, gunzipper)
	case strings.HasSuffix(filename, ".tar.bz2"), strings.HasSuffix(filename, ".tbz"):
		return NewArchiveExtractor(chooser, NewTarArchive, b2unzipper)
	case strings.HasSuffix(filename, ".tar.xz"), strings.HasSuffix(filename, ".txz"):
		return NewArchiveExtractor(chooser, NewTarArchive, xunzipper)
	case strings.HasSuffix(filename, ".tar.zst"):
		return NewArchiveExtractor(chooser, NewTarArchive, zstdunzipper)
	case strings.HasSuffix(filename, ".tar"):
		return NewArchiveExtractor(chooser, NewTarArchive, nounzipper)
	case strings.HasSuffix(filename, ".zip"):
		return NewArchiveExtractor(chooser, NewZipArchive, nil)
	case strings.HasSuffix(filename, ".7z"):
		return NewArchiveExtractor(chooser, NewSevenZipArchive, nil)
	case strings.HasSuffix(filename, ".gz"):
		return &SingleFileExtractor{Rename: tool, Name: filename, Decompress: gunzipper}
	case strings.HasSuffix(filename, ".bz2"):
		return &SingleFileExtractor{Rename: tool, Name: filename, Decompress: b2unzipper}
	case strings.HasSuffix(filename, ".xz"):
		return &SingleFileExtractor{Rename: tool, Name: filename, Decompress: xunzipper}
	case strings.HasSuffix(filename, ".zst"):
		return &SingleFileExtractor{Rename: tool, Name: filename, Decompress: zstdunzipper}
	default:
		return &SingleFileExtractor{Rename: tool, Name: filename, Decompress: nounzipper}
	}
}

func NewArchiveExtractor(file Chooser, ar ArchiveFn, decompress DecompFn) *ArchiveExtractor {
	return &ArchiveExtractor{File: file, Ar: ar, Decompress: decompress}
}

func NewDownloadOnlyExtractor(name string) *SingleFileExtractor {
	return &SingleFileExtractor{
		Name:   name,
		Rename: name,
		Decompress: func(r io.Reader) (io.Reader, error) {
			return r, nil
		},
	}
}

func NewBinaryChooser(tool string) *BinaryChooser {
	return &BinaryChooser{Tool: tool}
}

func NewGlobChooser(gl string) (*GlobChooser, error) {
	g, err := glob.Compile(gl, '/')
	return &GlobChooser{g: g, expr: gl, all: gl == "*" || gl == "/"}, err
}

func NewFileChooser(expr string) (Chooser, error) {
	parts := strings.Split(expr, ",")
	if len(parts) == 1 {
		return NewGlobChooser(strings.TrimSpace(expr))
	}

	choosers := make([]Chooser, 0, len(parts))
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		ch, err := NewGlobChooser(part)
		if err != nil {
			return nil, err
		}
		choosers = append(choosers, ch)
		normalized = append(normalized, part)
	}
	if len(choosers) == 0 {
		return nil, fmt.Errorf("empty file chooser expression")
	}
	if len(choosers) == 1 {
		return choosers[0], nil
	}
	return &MultiChooser{
		expr:     strings.Join(normalized, ","),
		choosers: choosers,
	}, nil
}

func (a *ArchiveExtractor) Extract(data []byte, multiple bool) (ExtractedFile, []ExtractedFile, error) {
	var candidates []ExtractedFile
	var dirs []string
	ar, err := a.Ar(data, a.Decompress)
	if err != nil {
		return ExtractedFile{}, nil, err
	}
	for {
		f, err := ar.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ExtractedFile{}, nil, fmt.Errorf("extract: %w", err)
		}
		var hasdir bool
		for _, d := range dirs {
			if strings.HasPrefix(f.Name, d) {
				hasdir = true
				break
			}
		}
		if hasdir {
			continue
		}
		direct, possible := a.File.Choose(f.Name, f.Dir(), f.Mode)
		if direct || possible {
			name := rename(f.Name, f.Name)
			fdata, err := ar.ReadAll()
			if err != nil {
				return ExtractedFile{}, nil, fmt.Errorf("extract: %w", err)
			}
			var extract func(to string) error
			if !f.Dir() {
				extract = func(to string) error {
					return writeFile(fdata, to, modeFrom(name, f.Mode))
				}
			} else {
				dirs = append(dirs, f.Name)
				extract = func(to string) error {
					subAr, err := a.Ar(data, a.Decompress)
					if err != nil {
						return err
					}
					type link struct {
						newname, oldname string
						sym              bool
					}
					var links []link
					for {
						subf, err := subAr.Next()
						if err == io.EOF {
							break
						}
						if err != nil {
							return fmt.Errorf("extract: %w", err)
						}
						rel, ok := archiveChildPath(f.Name, subf.Name)
						if !ok {
							continue
						}
						if rel == "" {
							if subf.Dir() {
								os.MkdirAll(to, 0o755)
							}
							continue
						}
						if subf.Dir() {
							dir, err := safeArchiveOutputPath(to, rel)
							if err != nil {
								return fmt.Errorf("extract: %w", err)
							}
							os.MkdirAll(dir, 0o755)
							continue
						}
						if subf.Type == TypeLink || subf.Type == TypeSymlink {
							newname, err := safeArchiveOutputPath(to, rel)
							if err != nil {
								return fmt.Errorf("extract: %w", err)
							}
							if err := validateArchiveLinkTarget(subf.LinkName); err != nil {
								return fmt.Errorf("extract: %w", err)
							}
							links = append(links, link{
								newname: newname,
								oldname: subf.LinkName,
								sym:     subf.Type == TypeSymlink,
							})
							continue
						}
						subData, err := subAr.ReadAll()
						if err != nil {
							return fmt.Errorf("extract: %w", err)
						}
						name, err := safeArchiveOutputPath(to, rel)
						if err != nil {
							return fmt.Errorf("extract: %w", err)
						}
						if err := writeFile(subData, name, subf.Mode); err != nil {
							return fmt.Errorf("extract: %w", err)
						}
					}
					for _, l := range links {
						os.Remove(l.newname)
						os.MkdirAll(filepath.Dir(l.newname), 0o755)
						var err error
						if l.sym {
							err = os.Symlink(l.oldname, l.newname)
						} else {
							oldname, pathErr := safeArchiveOutputPath(to, l.oldname)
							if pathErr != nil {
								return fmt.Errorf("extract: %w", pathErr)
							}
							err = os.Link(oldname, l.newname)
						}
						if err != nil && err != os.ErrExist {
							return fmt.Errorf("extract: %w", err)
						}
					}
					return nil
				}
			}
			ef := ExtractedFile{Name: name, ArchiveName: f.Name, mode: f.Mode, Extract: extract, Dir: f.Dir()}
			if direct && !multiple {
				return ef, nil, nil
			}
			candidates = append(candidates, ef)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil, nil
	}
	if len(candidates) == 0 {
		return ExtractedFile{}, candidates, fmt.Errorf("target %v not found in archive", a.File)
	}
	return ExtractedFile{}, candidates, fmt.Errorf("%d candidates for target %v found", len(candidates), a.File)
}

func (s *SingleFileExtractor) Extract(data []byte, multiple bool) (ExtractedFile, []ExtractedFile, error) {
	name := rename(s.Name, s.Rename)
	return ExtractedFile{
		Name:        name,
		ArchiveName: s.Name,
		mode:        0o666,
		Extract: func(to string) error {
			r := bytes.NewReader(data)
			dr, err := s.Decompress(r)
			if err != nil {
				return err
			}
			decdata, err := io.ReadAll(dr)
			if err != nil {
				return err
			}
			return writeFile(decdata, to, modeFrom(name, 0o666))
		},
	}, nil, nil
}

func archiveChildPath(parent, name string) (string, bool) {
	parent = archivePathForCompare(parent)
	name = archivePathForCompare(name)
	if parent != "" && !strings.HasSuffix(parent, "/") {
		parent += "/"
	}
	if !strings.HasPrefix(name, parent) {
		return "", false
	}
	return strings.TrimPrefix(name, parent), true
}

func archivePathForCompare(name string) string {
	return strings.ReplaceAll(name, `\`, "/")
}

func safeArchiveRelativePath(name string) (string, error) {
	name = archivePathForCompare(name)
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if cleanName == "." || filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) || cleanName == ".." || filepath.VolumeName(cleanName) != "" {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return cleanName, nil
}

func validateArchiveLinkTarget(name string) error {
	_, err := safeArchiveRelativePath(name)
	return err
}

func safeArchiveLinkName(name string, typ FileType) (string, error) {
	if typ != TypeLink && typ != TypeSymlink || name == "" {
		return name, nil
	}
	cleanName, err := safeArchiveRelativePath(name)
	if err != nil {
		return "", err
	}
	return cleanName, nil
}

func (b *BinaryChooser) Choose(name string, dir bool, mode fs.FileMode) (bool, bool) {
	if dir {
		return false, false
	}
	fmatch := filepath.Base(name) == b.Tool || filepath.Base(name) == b.Tool+".exe" || filepath.Base(name) == b.Tool+".appimage"
	possible := !mode.IsDir() && isExec(name, mode.Perm())
	return fmatch && possible, possible
}

func (b *BinaryChooser) String() string {
	return fmt.Sprintf("exe `%s`", b.Tool)
}

func (l *LiteralFileChooser) Choose(name string, dir bool, mode fs.FileMode) (bool, bool) {
	return false, filepath.Base(name) == filepath.Base(l.File) && strings.HasSuffix(name, l.File)
}

func (l *LiteralFileChooser) String() string {
	return fmt.Sprintf("`%s`", l.File)
}

func (g *GlobChooser) Choose(name string, dir bool, mode fs.FileMode) (bool, bool) {
	if g.all {
		return true, true
	}
	if len(name) > 0 && name[len(name)-1] == '/' {
		name = name[:len(name)-1]
	}
	return false, g.g.Match(filepath.Base(name)) || g.g.Match(name)
}

func (g *GlobChooser) String() string {
	return fmt.Sprintf("`%s`", g.expr)
}

func (m *MultiChooser) Choose(name string, dir bool, mode fs.FileMode) (bool, bool) {
	for _, chooser := range m.choosers {
		direct, possible := chooser.Choose(name, dir, mode)
		if direct || possible {
			return direct, true
		}
	}
	return false, false
}

func (m *MultiChooser) String() string {
	return fmt.Sprintf("`%s`", m.expr)
}

func tarft(typ byte) FileType {
	switch typ {
	case tar.TypeReg:
		return TypeNormal
	case tar.TypeDir:
		return TypeDir
	case tar.TypeLink:
		return TypeLink
	case tar.TypeSymlink:
		return TypeSymlink
	default:
		return TypeOther
	}
}

func NewTarArchive(data []byte, decompress DecompFn) (Archive, error) {
	r := bytes.NewReader(data)
	dr, err := decompress(r)
	if err != nil {
		return nil, err
	}
	return &TarArchive{r: tar.NewReader(dr)}, nil
}

func (t *TarArchive) Next() (File, error) {
	for {
		hdr, err := t.r.Next()
		if err != nil {
			return File{}, err
		}
		ft := tarft(hdr.Typeflag)
		if ft != TypeOther {
			name, err := safeArchiveRelativePath(hdr.Name)
			if err != nil {
				return File{}, err
			}
			linkName, err := safeArchiveLinkName(hdr.Linkname, ft)
			if err != nil {
				return File{}, err
			}
			return File{Name: name, LinkName: linkName, Mode: fs.FileMode(hdr.Mode), Type: ft}, nil
		}
	}
}

func (t *TarArchive) ReadAll() ([]byte, error) {
	return io.ReadAll(t.r)
}

func NewZipArchive(data []byte, d DecompFn) (Archive, error) {
	r := bytes.NewReader(data)
	zr, err := zip.NewReader(r, int64(len(data)))
	return &ZipArchive{r: zr, idx: -1}, err
}

type SevenZipArchive struct {
	r   *sevenzip.Reader
	idx int
}

func NewSevenZipArchive(data []byte, d DecompFn) (Archive, error) {
	r := bytes.NewReader(data)
	szr, err := sevenzip.NewReader(r, int64(len(data)))
	return &SevenZipArchive{r: szr, idx: -1}, err
}

func (z *ZipArchive) Next() (File, error) {
	z.idx++
	if z.idx < 0 || z.idx >= len(z.r.File) {
		return File{}, io.EOF
	}
	f := z.r.File[z.idx]
	typ := TypeNormal
	if strings.HasSuffix(f.Name, "/") {
		typ = TypeDir
	}
	name, err := safeArchiveRelativePath(f.Name)
	if err != nil {
		return File{}, err
	}
	return File{Name: name, Mode: f.Mode(), Type: typ}, nil
}

func (z *ZipArchive) ReadAll() ([]byte, error) {
	if z.idx < 0 || z.idx >= len(z.r.File) {
		return nil, io.EOF
	}
	f := z.r.File[z.idx]
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("zip extract: %w", err)
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func (z *SevenZipArchive) Next() (File, error) {
	z.idx++
	if z.idx < 0 || z.idx >= len(z.r.File) {
		return File{}, io.EOF
	}
	f := z.r.File[z.idx]
	mode := f.Mode()
	typ := TypeNormal
	if mode.IsDir() {
		typ = TypeDir
	}
	name, err := safeArchiveRelativePath(f.Name)
	if err != nil {
		return File{}, err
	}
	return File{Name: name, Mode: mode, Type: typ}, nil
}

func (z *SevenZipArchive) ReadAll() ([]byte, error) {
	if z.idx < 0 || z.idx >= len(z.r.File) {
		return nil, io.EOF
	}
	f := z.r.File[z.idx]
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("7z extract: %w", err)
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func writeFile(data []byte, rename string, mode fs.FileMode) error {
	if rename[0] == '-' {
		_, err := os.Stdout.Write(data)
		return err
	}
	os.Remove(rename)
	os.MkdirAll(filepath.Dir(rename), 0o755)
	f, err := os.OpenFile(rename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func modeFrom(fname string, mode fs.FileMode) fs.FileMode {
	if isExec(fname, mode) {
		return mode | 0o111
	}
	return mode
}

func rename(file string, nameguess string) string {
	if isDefinitelyNotExec(file) {
		return file
	}
	switch {
	case strings.HasSuffix(file, ".appimage"):
		return file[:len(file)-len(".appimage")]
	case strings.HasSuffix(file, ".exe"):
		return file
	default:
		return nameguess
	}
}

func isDefinitelyNotExec(file string) bool {
	return strings.HasSuffix(file, ".deb") || strings.HasSuffix(file, ".1") || strings.HasSuffix(file, ".txt")
}

func isExec(file string, mode os.FileMode) bool {
	if isDefinitelyNotExec(file) {
		return false
	}
	return strings.HasSuffix(file, ".exe") || strings.HasSuffix(file, ".appimage") || !strings.Contains(file, ".") || mode&0o111 != 0
}

var (
	installOSDarwin    = systemOS{name: "darwin", regex: regexp.MustCompile(`(?i)(darwin|mac.?(os)?|osx)`)}
	installOSWindows   = systemOS{name: "windows", regex: regexp.MustCompile(`(?i)([^r]win|windows)`)}
	installOSLinux     = systemOS{name: "linux", regex: regexp.MustCompile(`(?i)(linux|ubuntu)`), anti: regexp.MustCompile(`(?i)(android)`), priority: regexp.MustCompile(`\.appimage$`)}
	installOSNetBSD    = systemOS{name: "netbsd", regex: regexp.MustCompile(`(?i)(netbsd)`)}
	installOSFreeBSD   = systemOS{name: "freebsd", regex: regexp.MustCompile(`(?i)(freebsd)`)}
	installOSOpenBSD   = systemOS{name: "openbsd", regex: regexp.MustCompile(`(?i)(openbsd)`)}
	installOSAndroid   = systemOS{name: "android", regex: regexp.MustCompile(`(?i)(android)`)}
	installOSIllumos   = systemOS{name: "illumos", regex: regexp.MustCompile(`(?i)(illumos)`)}
	installOSSolaris   = systemOS{name: "solaris", regex: regexp.MustCompile(`(?i)(solaris)`)}
	installOSPlan9     = systemOS{name: "plan9", regex: regexp.MustCompile(`(?i)(plan9)`)}
	installArchAMD64   = systemArch{name: "amd64", regex: regexp.MustCompile(`(?i)(x64|amd64|x86(-|_)?64)`)}
	installArchI386    = systemArch{name: "386", regex: regexp.MustCompile(`(?i)(x32|amd32|x86(-|_)?32|i?386)`)}
	installArchArm     = systemArch{name: "arm", regex: regexp.MustCompile(`(?i)(arm32|armv6|arm\b)`)}
	installArchArm64   = systemArch{name: "arm64", regex: regexp.MustCompile(`(?i)(arm64|armv8|aarch64)`)}
	installArchRiscv64 = systemArch{name: "riscv64", regex: regexp.MustCompile(`(?i)(riscv64)`)}
)

var installGOOSMap = map[string]systemOS{
	"darwin":  installOSDarwin,
	"windows": installOSWindows,
	"linux":   installOSLinux,
	"netbsd":  installOSNetBSD,
	"openbsd": installOSOpenBSD,
	"freebsd": installOSFreeBSD,
	"android": installOSAndroid,
	"illumos": installOSIllumos,
	"solaris": installOSSolaris,
	"plan9":   installOSPlan9,
}

var installGOARCHMap = map[string]systemArch{
	"amd64":   installArchAMD64,
	"386":     installArchI386,
	"arm":     installArchArm,
	"arm64":   installArchArm64,
	"riscv64": installArchRiscv64,
}
