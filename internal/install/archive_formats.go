package install

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bodgit/sevenzip"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

type TarArchive struct {
	r *tar.Reader
}

type ZipArchive struct {
	r   *zip.Reader
	idx int
}

type SevenZipArchive struct {
	r   *sevenzip.Reader
	idx int
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
			if ft == TypeDir && archiveRootDirectoryEntry(hdr.Name) {
				continue
			}
			name, err := safeArchiveRelativePath(hdr.Name)
			if err != nil {
				return File{}, err
			}
			linkName, err := safeArchiveLinkName(hdr.Linkname, ft)
			if err != nil {
				return File{}, err
			}
			return File{Name: name, LinkName: linkName, Mode: fs.FileMode(hdr.Mode), ModTime: hdr.ModTime, Type: ft}, nil
		}
	}
}

func (t *TarArchive) ReadAll() ([]byte, error) {
	return io.ReadAll(t.r)
}

func (t *TarArchive) WriteTo(w io.Writer) (int64, error) {
	return io.Copy(w, t.r)
}

func NewZipArchive(data []byte, d DecompFn) (Archive, error) {
	r := bytes.NewReader(data)
	zr, err := zip.NewReader(r, int64(len(data)))
	return &ZipArchive{r: zr, idx: -1}, err
}

func NewSevenZipArchive(data []byte, d DecompFn) (Archive, error) {
	r := bytes.NewReader(data)
	szr, err := sevenzip.NewReader(r, int64(len(data)))
	return &SevenZipArchive{r: szr, idx: -1}, err
}

func (z *ZipArchive) Next() (File, error) {
	for {
		z.idx++
		if z.idx < 0 || z.idx >= len(z.r.File) {
			return File{}, io.EOF
		}
		f := z.r.File[z.idx]
		typ := TypeNormal
		if strings.HasSuffix(f.Name, "/") || strings.HasSuffix(f.Name, `\`) || f.FileInfo().IsDir() {
			typ = TypeDir
		}
		rawName := zipFileName(f)
		if typ == TypeDir && archiveRootDirectoryEntry(rawName) {
			continue
		}
		name, err := safeArchiveRelativePath(rawName)
		if err != nil {
			return File{}, err
		}
		return File{Name: name, Mode: f.Mode(), ModTime: zipModifiedTime(f), Type: typ}, nil
	}
}

func zipModifiedTime(f *zip.File) time.Time {
	if f == nil {
		return time.Time{}
	}
	if zipHasExtendedModTime(f.Extra) {
		return f.Modified.UTC()
	}
	modified := f.Modified
	return time.Date(modified.Year(), modified.Month(), modified.Day(), modified.Hour(), modified.Minute(), modified.Second(), modified.Nanosecond(), time.Local)
}

func zipHasExtendedModTime(extra []byte) bool {
	for len(extra) >= 4 {
		id := uint16(extra[0]) | uint16(extra[1])<<8
		size := int(uint16(extra[2]) | uint16(extra[3])<<8)
		extra = extra[4:]
		if size > len(extra) {
			return false
		}
		field := extra[:size]
		switch {
		case id == 0x000a && len(field) >= 32:
			return true
		case (id == 0x000d || id == 0x5855) && len(field) >= 8:
			return true
		case id == 0x5455 && len(field) >= 5 && field[0]&1 != 0:
			return true
		}
		extra = extra[size:]
	}
	return false
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

func (z *ZipArchive) WriteTo(w io.Writer) (int64, error) {
	if z.idx < 0 || z.idx >= len(z.r.File) {
		return 0, io.EOF
	}
	f := z.r.File[z.idx]
	rc, err := f.Open()
	if err != nil {
		return 0, fmt.Errorf("zip extract: %w", err)
	}
	defer rc.Close()
	return io.Copy(w, rc)
}

func (z *SevenZipArchive) Next() (File, error) {
	for {
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
		if typ == TypeDir && archiveRootDirectoryEntry(f.Name) {
			continue
		}
		name, err := safeArchiveRelativePath(f.Name)
		if err != nil {
			return File{}, err
		}
		return File{Name: name, Mode: mode, ModTime: sevenZipModTime(f), Type: typ}, nil
	}
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

func (z *SevenZipArchive) WriteTo(w io.Writer) (int64, error) {
	if z.idx < 0 || z.idx >= len(z.r.File) {
		return 0, io.EOF
	}
	f := z.r.File[z.idx]
	rc, err := f.Open()
	if err != nil {
		return 0, fmt.Errorf("7z extract: %w", err)
	}
	defer rc.Close()
	return io.Copy(w, rc)
}

func sevenZipModTime(f *sevenzip.File) time.Time {
	return f.FileHeader.Modified.UTC()
}

func zipFileName(f *zip.File) string {
	if f == nil || !f.NonUTF8 || utf8.ValidString(f.Name) {
		return f.Name
	}
	raw := []byte(f.Name)
	bestName := f.Name
	bestScore := legacyZipNameScore(f.Name)
	for _, enc := range []*charmap.Charmap{charmap.CodePage866, charmap.Windows1251} {
		name, _, err := transform.String(enc.NewDecoder(), string(raw))
		if err == nil && utf8.ValidString(name) {
			if score := legacyZipNameScore(name); score > bestScore {
				bestName = name
				bestScore = score
			}
		}
	}
	return bestName
}

func archiveRootDirectoryEntry(name string) bool {
	name = strings.ReplaceAll(name, `\`, "/")
	return path.Clean(strings.Trim(name, "/")) == "."
}

func legacyZipNameScore(name string) int {
	score := 0
	for _, r := range name {
		switch {
		case r == utf8.RuneError:
			score -= 20
		case unicode.Is(unicode.Cyrillic, r):
			score += 4
		case r >= 0x2500 && r <= 0x257F:
			score -= 6
		case r < 0x20:
			score -= 10
		case r < utf8.RuneSelf:
			score++
		}
	}
	return score
}
