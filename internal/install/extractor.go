package install

import (
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"io"
	"io/fs"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

type Extractor interface {
	Extract(source io.ReadSeeker, size int64, multiple bool) (ExtractedFile, []ExtractedFile, error)
}

type DirectAllExtractor interface {
	ExtractAllTo(source io.ReadSeeker, size int64, output string) ([]string, error)
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

type SingleFileExtractor struct {
	Rename     string
	Name       string
	Decompress func(r io.Reader) (io.Reader, error)
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

func NewDownloadOnlyExtractor(name string) *SingleFileExtractor {
	return &SingleFileExtractor{
		Name:   name,
		Rename: name,
		Decompress: func(r io.Reader) (io.Reader, error) {
			return r, nil
		},
	}
}

func (s *SingleFileExtractor) Extract(source io.ReadSeeker, size int64, multiple bool) (ExtractedFile, []ExtractedFile, error) {
	name := rename(s.Name, s.Rename)
	return ExtractedFile{
		Name:        name,
		ArchiveName: s.Name,
		mode:        0o666,
		Extract: func(to string) error {
			if _, err := source.Seek(0, io.SeekStart); err != nil {
				return err
			}
			dr, err := s.Decompress(source)
			if err != nil {
				return err
			}
			return writeReaderWithModTime(dr, to, modeFrom(name, 0o666), time.Time{})
		},
	}, nil, nil
}
